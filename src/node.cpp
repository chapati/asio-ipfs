#include <ipfs_bindings.h>
#include <asio_ipfs/error.h>
#include <assert.h>
#include <mutex>
#include <experimental/tuple>
#include <boost/intrusive/list.hpp>
#include <boost/optional.hpp>

#include <asio_ipfs.h>

using namespace asio_ipfs;
using namespace std;
namespace asio = boost::asio;
namespace sys  = boost::system;
namespace intr = boost::intrusive;

static const uint64_t INVALID_IPFS_HANDLE = uint64_t(-1);
static const uint64_t INVALID_CANCEL_SIGNAL_ID = uint64_t(-1);

template<class F> struct Defer {
    F f;
    ~Defer() { f(); }
};

template<class F> Defer<F> defer(F&& f) {
    return Defer<F>{forward<F>(f)};
};

struct HandleBase : public intr::list_base_hook
                            <intr::link_mode<intr::auto_unlink>> {
    virtual void cancel() = 0;
    virtual ~HandleBase() { }
};

struct asio_ipfs::node_impl {
    uint64_t ipfs_handle = INVALID_IPFS_HANDLE;
    // This prevents callbacks from being called once the node is destroyed.
    bool was_destroyed;
    asio::io_service& ios;
    mutex mu;
    intr::list<HandleBase, intr::constant_time_size<false>> handles;

    node_impl(asio::io_service& ios)
        : was_destroyed(false)
        , ios(ios)
    {}
};

template<class... As>
struct Handle : public HandleBase {
    shared_ptr<node_impl> impl;
    function<void(sys::error_code, As&&...)> cb;
    function<void()>* cancel_fn;
    boost::optional<asio::io_service::work> work;
    tuple<sys::error_code, As...> args;
    bool was_canceled = false;

    Handle( shared_ptr<node_impl> impl_
          , uint64_t cancel_signal_id
          , function<void()>* cancel_fn
          , function<void(sys::error_code, As&&...)> cb)
        : impl(move(impl_))
        , cb(move(cb))
        , cancel_fn(cancel_fn)
        , work(asio::io_service::work(impl->ios))
    {
        impl->handles.push_back(*this);

        if (cancel_fn) {
            *this->cancel_fn = [ h = impl->ipfs_handle
                               , id = cancel_signal_id
                               , &mu = impl->mu
                               , &was_canceled = was_canceled] {
                lock_guard<mutex> guard(mu);

                if (was_canceled) return;
                was_canceled = true;
                go_asio_ipfs_cancel(h, id);
            };
        }
    }

    static void call(int err, void* arg, As... args) {
        auto self = reinterpret_cast<Handle*>(arg);
        auto& ios = self->impl->ios;

        lock_guard<mutex> guard(self->impl->mu);

        // Already cancelled? (by destroying asio_ipfs::node)
        if (!self->cb) { delete self; return; }

        self->unlink();

        self->args = make_tuple( make_error_code(error::ipfs_error{err})
                               , move(args)...);

        ios.dispatch([self] {
            auto on_exit = defer([=] { delete self; });
            if (self->impl->was_destroyed) {
                // cb has already been scheduled for execution from ~node()
                return;
            }
            if (self->was_canceled) {
                std::get<0>(self->args) = asio::error::operation_aborted;
            }
            if (self->cancel_fn) *self->cancel_fn = nullptr;
            std::experimental::apply(self->cb, move(self->args));
        });
    }

    static void call_void(int err, void* arg) {
        call(err, arg);
    }

    static void call_data(int err, const char* data, size_t size, void* arg) {
        call(err, arg, string(data, data + size));
    }

    static void call_uint64(int err, const char* data, size_t size, void* arg) {
        assert(size == sizeof(uint64_t));
        call(err, arg, *reinterpret_cast<const uint64_t*>(data));
    }

    void cancel() override {
        work = boost::none;
        std::get<0>(args) = asio::error::operation_aborted;

        impl->ios.post([cb = move(cb), as = move(args)] () mutable {
                std::experimental::apply(cb, move(as));
            });
    }
};

void node::build_( asio::io_service& ios
                 , const string& repo_path
                 , function<void( const sys::error_code& ec
                                , unique_ptr<node>)> cb)
{
    auto impl = make_shared<node_impl>(ios);

    auto cb_ = [cb = move(cb), impl]
               (const sys::error_code& ec, uint64_t ipfs_handle) {
        if (ec) return cb(ec, nullptr);
        impl->ipfs_handle = ipfs_handle;
        cb(ec, unique_ptr<node>(new node(move(impl))));
    };

    go_asio_ipfs_async_start( (char*) repo_path.data()
                            , (void*) Handle<uint64_t>::call_uint64
                            , (void*) new Handle<uint64_t>{ impl
                                                          , INVALID_CANCEL_SIGNAL_ID
                                                          , nullptr
                                                          , move(cb_)});
}

node::node(shared_ptr<node_impl> impl)
    : _impl(move(impl))
{
}

node::node(asio::io_service& ios, const string& repo_path)
    : _impl(make_shared<node_impl>(ios))
{
    int ec = go_asio_ipfs_start((char*) repo_path.data(), &_impl->ipfs_handle);

    if (ec != IPFS_SUCCESS) {
        throw std::runtime_error("node: Failed to start IPFS");
    }
}

string node::id() const {
    char* cid = go_asio_ipfs_node_id(_impl->ipfs_handle);
    string ret(cid);
    free(cid);
    return ret;
}

void node::publish_( const string& cid
                   , Timer::duration d
                   , Cancel* cancel
                   , std::function<void(sys::error_code)> cb)
{
    using namespace std::chrono;

    assert(cid.size() == CID_SIZE);
    assert(_impl->ipfs_handle != INVALID_IPFS_HANDLE);

    uint64_t cancel_signal_id
        = go_asio_ipfs_make_unique_cancel_signal_id(_impl->ipfs_handle);

    go_asio_ipfs_publish( _impl->ipfs_handle
                        , cancel_signal_id
                        , (char*) cid.data()
                        , duration_cast<seconds>(d).count()
                        , (void*) Handle<>::call_void
                        , (void*) new Handle<>{ _impl
                                              , cancel_signal_id
                                              , cancel
                                              , move(cb)});
}

void node::resolve_( const string& node_id
                   , Cancel* cancel
                   , function<void(sys::error_code, string)> cb)
{
    assert(_impl->ipfs_handle != INVALID_IPFS_HANDLE);

    uint64_t cancel_signal_id
        = go_asio_ipfs_make_unique_cancel_signal_id(_impl->ipfs_handle);

    go_asio_ipfs_resolve( _impl->ipfs_handle
                        , cancel_signal_id
                        , (char*) node_id.data()
                        , (void*) Handle<string>::call_data
                        , (void*) new Handle<string>{ _impl
                                                    , cancel_signal_id
                                                    , cancel
                                                    , move(cb)} );
}

void node::add_( const uint8_t* data
               , size_t size
               , function<void(sys::error_code, string)> cb)
{
    assert(_impl->ipfs_handle != INVALID_IPFS_HANDLE);

    go_asio_ipfs_add( _impl->ipfs_handle
                    , (void*) data, size
                    , (void*) Handle<string>::call_data
                    , (void*) new Handle<string>{ _impl
                                                , INVALID_CANCEL_SIGNAL_ID
                                                , nullptr
                                                , move(cb)} );
}

void node::cat_( const string& cid
               , Cancel* cancel
               , function<void(sys::error_code, string)> cb)
{
    assert(cid.size() == CID_SIZE);
    assert(_impl->ipfs_handle != INVALID_IPFS_HANDLE);

    uint64_t cancel_signal_id
        = go_asio_ipfs_make_unique_cancel_signal_id(_impl->ipfs_handle);

    go_asio_ipfs_cat( _impl->ipfs_handle
                    , cancel_signal_id
                    , (char*) cid.data()
                    , (void*) Handle<string>::call_data
                    , (void*) new Handle<string>{ _impl
                                                , cancel_signal_id
                                                , cancel
                                                , move(cb)} );
}

void node::pin_( const string& cid
               , Cancel* cancel
               , std::function<void(sys::error_code)> cb)
{
    assert(cid.size() == CID_SIZE);
    assert(_impl->ipfs_handle != INVALID_IPFS_HANDLE);

    uint64_t cancel_signal_id
        = go_asio_ipfs_make_unique_cancel_signal_id(_impl->ipfs_handle);

    go_asio_ipfs_pin( _impl->ipfs_handle
                    , cancel_signal_id
                    , (char*) cid.data()
                    , (void*) Handle<>::call_void
                    , (void*) new Handle<>{ _impl
                                          , cancel_signal_id
                                          , cancel
                                          , move(cb)});
}

void node::unpin_( const string& cid
                 , Cancel* cancel
                 , std::function<void(sys::error_code)> cb)
{
    assert(cid.size() == CID_SIZE);
    assert(_impl->ipfs_handle != INVALID_IPFS_HANDLE);

    uint64_t cancel_signal_id
        = go_asio_ipfs_make_unique_cancel_signal_id(_impl->ipfs_handle);

    go_asio_ipfs_unpin( _impl->ipfs_handle
                      , cancel_signal_id
                      , (char*) cid.data()
                      , (void*) Handle<>::call_void
                      , (void*) new Handle<>{ _impl
                                            , cancel_signal_id
                                            , cancel
                                            , move(cb)});
}

boost::asio::io_service& node::get_io_service()
{
    return _impl->ios;
}

node::~node()
{
    if (!_impl) return; // Was moved from.

    lock_guard<mutex> guard(_impl->mu);
    _impl->was_destroyed = true;

    // Make sure all handlers get completed.
    for (auto i = _impl->handles.begin(); i != _impl->handles.end();) {
        auto j = std::next(i);

        auto h = &(*i);
        h->cancel();
        h->unlink();

        i = j;
    }

    if (_impl->ipfs_handle != INVALID_IPFS_HANDLE) {
        go_asio_ipfs_stop(_impl->ipfs_handle);
    }
}
