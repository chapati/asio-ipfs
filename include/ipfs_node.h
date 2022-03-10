#pragma once
#include <string>
#include <functional>
#include <memory>
#include <boost/asio/steady_timer.hpp>
#include <boost/asio/io_service.hpp>
#include <boost/utility/string_view.hpp>
#include "ipfs_config.h"

namespace asio_ipfs {
    struct node_impl;
    class node {
        using Timer = boost::asio::steady_timer;
        using Cancel = std::function<void()>;

        template<class Token, class... Ret>
        using Result = typename boost::asio::async_result<std::decay_t<Token>, void(boost::system::error_code, Ret...)>;

        template<class Token, class... Ret>
        using Handler = typename boost::asio::async_result<std::decay_t<Token>, void(boost::system::error_code, Ret...)>::completion_handler_type;

        using string_view = boost::string_view;

    public:
        static const uint32_t CID_SIZE = 46;
        using StateCB = std::function<void (const std::string& error, uint32_t peercnt)>;

        using LogCB = std::function<void (const char*)>;

        [[maybe_unused]] static void redirect_logs(LogCB);

    public:
        // This constructor may do repository initialization disk IO and as such
        // may block for a second or more. If that is undesired, use the static
        // async `node::build` function instead.
        node(boost::asio::io_service&, StateCB, config);

        node(node&&) noexcept;
        node& operator=(node&&) noexcept;

        node(const node&) = delete;
        node& operator=(const node&) = delete;

        boost::asio::io_service& get_io_service();
        void free();
        ~node();

        template<class Token>
        static typename Result<Token, std::unique_ptr<node>>::return_type
        build(boost::asio::io_service&, StateCB, config, Token&&);

        template<class Token>
        static typename Result<Token, std::unique_ptr<node>>::return_type
        build(boost::asio::io_service&, StateCB, config, Cancel&, Token&&);

        // Returns this node's IPFS ID
        [[nodiscard]] std::string id() const;

        template<class Token>
        typename Result<Token, std::string>::return_type
        add(const uint8_t* data, size_t size, bool pin, Token&&);

        template<class Token>
        typename Result<Token, std::string>::return_type
        add(const uint8_t* data, size_t size, bool pin, Cancel&, Token&&);

        template<class Token>
        typename Result<Token, std::string>::return_type
        calc_cid(const uint8_t* data, size_t size, Token&&);

        template<class Token>
        typename Result<Token, std::string>::return_type
        calc_cid(const uint8_t* data, size_t size, Cancel&, Token&&);

        template<class Token>
        typename Result<Token, std::vector<uint8_t>>::return_type
        cat(const std::string& cid, Token&&);

        template<class Token>
        typename Result<Token, std::vector<uint8_t>>::return_type
        cat(const std::string& cid, Cancel& cancel, Token&&);

        template<class Token>
        void publish(const std::string& cid, Timer::duration, Token&&);

        template<class Token>
        void publish(const std::string& cid, Timer::duration, Cancel&, Token&&);

        template<class Token>
        typename Result<Token, std::string>::return_type
        resolve(const std::string& ipns_id, Token&&);

        template<class Token>
        typename Result<Token, std::string>::return_type
        resolve(const std::string& ipns_id, Cancel&, Token&&);

        template<class Token>
        void pin(const std::string& cid, Token&&);

        template<class Token>
        void pin(const std::string& cid, Cancel&, Token&&);

        template<class Token>
        void unpin(const std::string& cid, Token&&);

        template<class Token>
        void unpin(const std::string& cid, Cancel&, Token&&);

        template<class Token>
        void gc(Cancel&, Token&&);

    private:
        static void build_(boost::asio::io_service&, StateCB, config, Cancel*,
                           std::function<void( const boost::system::error_code&, std::unique_ptr<node>)>);
        void add_(const uint8_t* data, size_t size, bool pin, Cancel*, std::function<void(boost::system::error_code, std::string)>&&);
        void calc_cid_(const uint8_t* data, size_t size, Cancel*, std::function<void(boost::system::error_code, std::string)>&&);
        void cat_(const std::string& cid, Cancel*, std::function<void(boost::system::error_code, std::vector<uint8_t>)>);
        void publish_(const std::string& cid, Timer::duration, Cancel*, std::function<void(boost::system::error_code)>);
        void resolve_(const std::string& ipns_id, Cancel*, std::function<void(boost::system::error_code, std::string)>);
        void pin_(const std::string& cid, Cancel*, std::function<void(boost::system::error_code)>);
        void unpin_(const std::string& cid, Cancel*, std::function<void(boost::system::error_code)>);
        void gc_(Cancel*, std::function<void(boost::system::error_code)>);

    private:
        node();
        std::unique_ptr<node_impl> _impl;
    };

    template<class Token>
    inline typename node::Result<Token, std::unique_ptr<node>>::return_type
    node::build( boost::asio::io_service& ios
               , StateCB scb
               , config cfg
               , Token&& token)
    {
        using BackendP = std::unique_ptr<node>;
        Handler<Token, BackendP> handler(std::forward<Token>(token));
        Result<Token, BackendP> result(handler);
        build_(ios, move(scb), cfg, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::unique_ptr<node>>::return_type
    node::build( boost::asio::io_service& ios
               , StateCB scb
               , config cfg
               , Cancel& cancel
               , Token&& token)
    {
        using BackendP = std::unique_ptr<node>;
        Handler<Token, BackendP> handler(std::forward<Token>(token));
        Result<Token, BackendP> result(handler);
        build_(ios, move(scb), cfg, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::string>::return_type
    node::add(const uint8_t* data, size_t size, bool pin, Token&& token)
    {
        Handler<Token, std::string> handler(std::forward<Token>(token));
        Result<Token, std::string> result(handler);
        add_(data, size, pin, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::string>::return_type
    node::add(const uint8_t* data, size_t size, bool pin, Cancel& cancel, Token&& token)
    {
        Handler<Token, std::string> handler(std::forward<Token>(token));
        Result<Token, std::string> result(handler);
        add_(data, size, pin, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::string>::return_type
    node::calc_cid(const uint8_t* data, size_t size, Token&& token)
    {
        Handler<Token, std::string> handler(std::forward<Token>(token));
        Result<Token, std::string> result(handler);
        calc_cid_(data, size, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::string>::return_type
    node::calc_cid(const uint8_t* data, size_t size, Cancel& cancel, Token&& token)
    {
        Handler<Token, std::string> handler(std::forward<Token>(token));
        Result<Token, std::string> result(handler);
        calc_cid_(data, size, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::vector<uint8_t>>::return_type
    node::cat(const std::string& cid, Token&& token)
    {
        Handler<Token, std::vector<uint8_t>> handler(std::forward<Token>(token));
        Result<Token, std::vector<uint8_t>> result(handler);
        cat_(cid, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::vector<uint8_t>>::return_type
    node::cat(const std::string& cid, Cancel& cancel, Token&& token)
    {
        Handler<Token, std::vector<uint8_t>> handler(std::forward<Token>(token));
        Result<Token, std::vector<uint8_t>> result(handler);
        cat_(cid, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::publish(const std::string& cid, Timer::duration d, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        publish_(cid, d, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::publish(const std::string& cid, Timer::duration d, Cancel& cancel, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        publish_(cid, d, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::string>::return_type
    node::resolve(const std::string& ipns_id, Token&& token)
    {
        Handler<Token, std::string> handler(std::forward<Token>(token));
        Result<Token, std::string> result(handler);
        resolve_(ipns_id, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline typename node::Result<Token, std::string>::return_type
    node::resolve(const std::string& ipns_id, Cancel& cancel, Token&& token)
    {
        Handler<Token, std::string> handler(std::forward<Token>(token));
        Result<Token, std::string> result(handler);
        resolve_(ipns_id, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::pin(const std::string& cid, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        pin_(cid, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::pin(const std::string& cid, Cancel& cancel, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        pin_(cid, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::unpin(const std::string& cid, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        unpin_(cid, nullptr, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::unpin(const std::string& cid, Cancel& cancel, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        unpin_(cid, &cancel, std::move(handler));
        return result.get();
    }

    template<class Token>
    inline void node::gc(Cancel& cancel, Token&& token)
    {
        Handler<Token> handler(std::forward<Token>(token));
        Result<Token> result(handler);
        gc_(&cancel, std::move(handler));
        return result.get();
    }
}
