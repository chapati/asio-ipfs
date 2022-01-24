#ifdef WIN32
typedef size_t __SIZE_TYPE__;
#define _Complex
#include <ipfs-bindings.h>
#else
#include <libipfs-bindings.h>
#endif

#include <cassert>
#include <tuple>
#include <boost/intrusive/list.hpp>
#include <boost/optional.hpp>
#include <nlohmann/json.hpp>
#include <asio_ipfs.h>
#include <iostream>

using namespace asio_ipfs;
using namespace std;
namespace asio = boost::asio;
namespace sys  = boost::system;
namespace intr = boost::intrusive;

template<class F> struct Defer { F f; ~Defer() { f(); } };
template<class F> Defer<F> defer(F&& f) { return Defer<F>{forward<F>(f)}; }

struct HandleBase : public intr::list_base_hook
                            <intr::link_mode<intr::auto_unlink>> {
    virtual void cancel() = 0;
    virtual ~HandleBase() = default;
};

struct asio_ipfs::node_impl {
    asio::io_service& ios;
    intr::list<HandleBase, intr::constant_time_size<false>> handles;

    explicit node_impl(asio::io_service& ios)
        : ios(ios)
    {}
};

template<class... As>
struct Handle : public HandleBase {
    asio::io_service& ios;
    function<void(sys::error_code, As&&...)> cb;
    function<void()>* cancel_fn;
    function<void()> destructor_cancel_fn;
    boost::optional<uint64_t> cancel_signal_id;
    asio::io_service::work work;
    unsigned job_count = 1;

    Handle( node_impl* impl
          , boost::optional<uint64_t> cancel_signal_id_
          , function<void()>* cancel_fn_
          , function<void(sys::error_code, As&&...)> cb_)
        : ios(impl->ios)
        , cancel_fn(cancel_fn_ ? cancel_fn_ : &destructor_cancel_fn)
        , cancel_signal_id(cancel_signal_id_)
        , work(asio::io_service::work(ios))
    {
        impl->handles.push_back(*this);

        cb = [this, cb_ = std::move(cb_)] (sys::error_code ec, As... args) {
            (*cancel_fn) = []{};
            if (cancel_signal_id) {
                go_asio_ipfs_cancellation_free(*cancel_signal_id);
            }
            // We need to unlink here, otherwise the callback could invoke the
            // destructor, which would in turn call `cancel` and expect that it
            // gets unlinked. But we just set the `cancel_fn` to do nothing
            // above, so the destructor ends up in an infinite loop.
            unlink();
            std::apply(cb_, make_tuple(ec, std::move(args)...));
        };

        *cancel_fn = [this] {
            unlink();
            if (cancel_signal_id) {
                go_asio_ipfs_cancel(*cancel_signal_id);
            }

            assert(cb);
            assert(job_count);
            ++job_count;

            auto postcb = [this, callback = std::move(cb)](){
                auto on_exit = defer([&] { if (!--job_count) delete(this); });

                tuple<sys::error_code, As...> args;
                std::get<0>(args) = asio::error::operation_aborted;
                std::apply(callback, std::move(args));
            };

            this->ios.post(postcb);
            (*cancel_fn) = []{};
        };

        /*
         * Exactly one of cb and *cancel_fn is ever called, in the asio thread.
         */
    }

    /*
     * This function is always called, in a go thread. If the Handle was
     * cancelled, self->cb is empty.
     */
    static void call(int err, void* arg, As... args) {
        auto self = reinterpret_cast<Handle*>(arg);
        self->ios.post([
            self,
            full_args = make_tuple(make_error_code(error::ipfs_error{err}), std::move(args)...)
        ] {
            auto on_exit = defer([&] { if (!--self->job_count) delete(self); });

            if (self->cb) {
                std::apply(self->cb, tuple<sys::error_code, As...>(std::move(full_args)));
            }
        });
    }

    void cancel() override {
        (*cancel_fn)();
    }
};

template<class... As> struct callback_function;

template<> struct callback_function<> {
    static void callback(int err, void* arg) {
        Handle<>::call(err, arg);
    }
};

template<> struct callback_function<std::string> {
    static void callback(int err, const char* data, size_t size, void* arg) {
        Handle<std::string>::call(err, arg, std::string(data, data + size));
    }
};

template<> struct callback_function<std::vector<uint8_t>> {
    static void callback(int err, const uint8_t* data, size_t size, void* arg) {
        Handle<std::vector<uint8_t>>::call(err, arg, std::vector<uint8_t>(data, data + size));
    }
};

template<class... CbAs, class F, class... As>
void call_ipfs(
    node_impl* node,
    std::function<void()>* cancel,
    std::function<void(sys::error_code, CbAs...)> callback,
    F ipfs_function,
    As... args
) {
    uint64_t cancel_signal_id = go_asio_ipfs_cancellation_allocate();
    ipfs_function(
        cancel_signal_id,
        args...,
        (void*) &callback_function<CbAs...>::callback,
        (void*) (new Handle<CbAs...>{ node, cancel_signal_id, cancel, std::move(callback) })
    );
}

template<class... CbAs, class F, class... As>
void call_ipfs_nocancel(
    node_impl* node,
    std::function<void()>* cancel,
    std::function<void(sys::error_code, CbAs...)> callback,
    F ipfs_function,
    As... args
) {
    ipfs_function(
        args...,
        (void*) &callback_function<CbAs...>::callback,
        (void*) (new Handle<CbAs...>{ node, boost::none, cancel, std::move(callback) })
    );
}

static string config_to_json(config cfg)
{
    if (cfg.bootstrap.empty())
    {
        throw std::runtime_error("IPFS bootstrap cannot be empty");
    }

    auto json = nlohmann::json
    {
        {"RepoRoot",         cfg.repo_root},
        {"LowWater",         cfg.low_water},
        {"HighWater",        cfg.high_water},
        {"GracePeriod",      std::to_string(cfg.grace_period) + std::string("s")},
        {"Bootstrap",        cfg.bootstrap},
        {"SwarmPort",        cfg.swarm_port},
        {"APIPort",          cfg.api_port},
        {"GatewayPort",      cfg.gateway_port},
        {"DefaultProfile",   cfg.default_profile},
        {"AutoRelay",        cfg.auto_relay},
        {"RelayHop",         cfg.relay_hop},
        {"StorageMax",       cfg.storage_max},
        {"AutoNAT",          cfg.autonat},
        {"AutoNATLimit",     cfg.autonat_limit},
        {"AutoNATPeerLimit", cfg.autonat_peer_limit},
        {"SwarmKey",         cfg.swarm_key},
        {"RoutingType",      cfg.routing_type},
        {"RunGC",            cfg.run_gc},
    };

    return json.dump();
}

node::node(asio::io_service& ios, config cfg)
{
    string cfg_s = config_to_json(cfg);
    int ec = go_asio_ipfs_start_blocking((char*)cfg_s.c_str(), (char*) cfg.repo_root.data());

    if (ec != IPFS_SUCCESS) {
        auto err = error::make_error_code(error::ipfs_error{ec});
        throw std::runtime_error(err.message());
    }

    _impl = make_unique<node_impl>(ios);
}

void node::free() {
    if (_impl) {
        // Make sure all handlers get completed.
        while (!_impl->handles.empty()) {
            auto& e = _impl->handles.front();
            e.cancel();
        }

        auto ec = go_asio_ipfs_free();
        if (ec != IPFS_SUCCESS) {
            auto err = error::make_error_code(error::ipfs_error{ec});
            throw std::runtime_error(err.message());
        }

        _impl.reset();
    }
}

node::~node()
{
    try
    {
        free();
    }
    catch(std::runtime_error&)
    {
        assert(false);
    }
}

void node::build_( asio::io_service& ios
                 , config cfg
                 , Cancel* cancel
                 , function<void( const sys::error_code& ec
                                , unique_ptr<node>)> cb)
{
    /*
     * This cannot be a unique_ptr, because std::function wants to be
     * CopyConstructible for some reason.
     */
    auto impl = new node_impl(ios);
    std::function<void(sys::error_code)> cb_ = [cb = move(cb), impl] (sys::error_code ec) {
        if (ec) {
            delete impl;
            cb(ec, nullptr);
        } else {
            std::unique_ptr<node> node_(new node);
            node_->_impl = unique_ptr<node_impl>(impl);
            cb(ec, std::move(node_));
        }
    };

    string cfg_s = config_to_json(cfg);
    call_ipfs_nocancel( impl
                      , cancel
                      , cb_
                      , go_asio_ipfs_start_async, (char*) cfg_s.c_str()
                                                , (char*) cfg.repo_root.data());
}

node::node() = default;
node::node(node&&) noexcept = default;
node& node::operator=(node&&) noexcept = default;

string node::id() const {
    char* cid = go_asio_ipfs_node_id();
    string ret(cid);
    go_asio_memfree(cid);
    return ret;
}

void node::publish_( const string& cid
                   , Timer::duration d
                   , Cancel* cancel
                   , std::function<void(sys::error_code)> cb)
{
    assert(cid.size() == CID_SIZE);
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_publish, (char*) cid.data(), std::chrono::duration_cast<std::chrono::seconds>(d).count());
}

void node::resolve_( const string& node_id
                   , Cancel* cancel
                   , function<void(sys::error_code, string)> cb)
{
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_resolve, (char*) node_id.data());
}

void node::add_( const uint8_t* data
               , size_t size
               , Cancel* cancel
               , function<void(sys::error_code, string)>&& cb)
{
    call_ipfs_nocancel(_impl.get(), cancel, std::move(cb), go_asio_ipfs_add, (void*) data, size, false);
}

void node::calculate_cid_( const string_view data
                         , Cancel* cancel
                         , function<void(sys::error_code, string)> cb)
{
    const char* d = data.data();
    size_t s = data.size();
    call_ipfs_nocancel(_impl.get(), cancel, std::move(cb), go_asio_ipfs_add, (void*) d, s, true);
}

void node::cat_( string_view cid
               , Cancel* cancel
               , std::function<void(boost::system::error_code, std::vector<uint8_t>)> cb)
{
    assert(cid.size() == CID_SIZE);
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_cat, (char*) cid.data());
}

void node::cat_( string_view cid
               , Cancel* cancel
               , function<void(sys::error_code, string)> cb)
{
    assert(cid.size() == CID_SIZE);
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_cat, (char*) cid.data());
}

void node::pin_( const string& cid
               , Cancel* cancel
               , std::function<void(sys::error_code)> cb)
{
    assert(cid.size() == CID_SIZE);
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_pin, (char*) cid.data());
}

void node::unpin_( const string& cid
                 , Cancel* cancel
                 , std::function<void(sys::error_code)> cb)
{
    assert(cid.size() == CID_SIZE);
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_unpin, (char*) cid.data());
}

void node::gc_(Cancel* cancel, std::function<void(boost::system::error_code)> cb)
{
    call_ipfs(_impl.get(), cancel, std::move(cb), go_asio_ipfs_gc);
}

boost::asio::io_service& node::get_io_service()
{
    return _impl->ios;
}

