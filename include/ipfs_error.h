#pragma once

#include <string>
#include <boost/system/error_code.hpp>
#include "ipfs_error_codes.h"

namespace asio_ipfs::error {

    struct ipfs_error {
        int error_number;
    };

    enum error_t {
        db_download_failed = 1, // Start with > 0, because 0 means success.,
        invalid_db_format,
        malformed_db_entry,
        missing_ipfs_link,
    };

    struct ipfs_category : public boost::system::error_category
    {
        [[nodiscard]] const char* name() const noexcept override
        {
            return "ipfs_errors";
        }

        [[nodiscard]] std::string message(int e) const override
        {
            switch (e) {
                case IPFS_SUCCESS:
                    return "success";
                case IPFS_NODE_EXISTS:
                    return "node aleady exists";
                case IPFS_PARSE_CONFIG_FAIL:
                    return "failed to parse IPFS config";
                case IPFS_READ_CONFIG_FAILED:
                    return "failed to read config from IPFS respo";
                case IPFS_CREATE_REPO_FAILED:
                    return "failed to create IPFS repository";
                case IPFS_API_ACCESS_FAILED:
                    return "failed to access core API";
                case IPFS_START_NODE_FAILED:
                    return "failed to start IPFS node";
                case IPFS_MPROME_INJECT_FAILED:
                    return "failed to inject mprome";
                case IPFS_FLATFS_FAILED:
                    return "failt to launch flatfs plugin";
                case IPFS_LEVELDS_FAILED:
                    return "failt to launch lefelds plugin";
                case IPFS_NO_NODE:
                    return "failed to find a node";
                case IPFS_RESOLVE_FAILED:
                    return "failed to resolve IPNS entry";
                case IPFS_CALC_CID_FAILED:
                    return "failed to calculate CID";
                case IPFS_ADD_FAILED:
                    return "failed to add data";
                case IPFS_CAT_FAILED:
                    return "failed to get data reader";
                case IPFS_READ_FAILED:
                    return "failed to read data";
                case IPFS_PUBLISH_FAILED:
                    return "failed to publish CID";
                case IPFS_PIN_FAILED:
                    return "failed to pin";
                case IPFS_UNPIN_FAILED:
                    return "failed to unpin";
                case IPFS_GC_FAILED:
                    return "failed to garbage collect";
                case IPFS_API_FAILED:
                    return "failed to start API";
                case IPFS_GATEWAY_FAILED:
                    return "failed to start Gateway";
                case IPFS_MFSPIN_FAILED:
                    return "failes to start MFS pin service";
                case IPFS_BUILD_ENV_FAILED:
                    return "failed to build initital environment";
                case IPFS_OPEN_REPO_FAILED:
                    return "failed to open repo";
                case IPFS_PLUGINS_FAILED:
                    return "failed to start plugins";
                default:
                    return "unknown ipfs error";
            }
        }
    };

    struct asio_ipfs_category : public boost::system::error_category
    {
        [[nodiscard]] const char* name() const noexcept override
        {
            return "asio_ipfs_errors";
        }

        [[nodiscard]] std::string message(int e) const override
        {
            switch (e) {
                case error::db_download_failed:
                    return "database download failed";
                case error::invalid_db_format:
                    return "invalid database format";
                case error::malformed_db_entry:
                    return "malformed database entry";
                case error::missing_ipfs_link:
                    return "missing IPFS link to content";
                default:
                    return "unknown asio_ipfs error";
            }
        }
    };

    boost::system::error_code
    make_error_code(::asio_ipfs::error::ipfs_error);

    boost::system::error_code
    make_error_code(::asio_ipfs::error::error_t);

} // asio_ipfs::error namespace

namespace boost::system {
    template<>
    struct is_error_code_enum<::asio_ipfs::error::error_t>
        : public std::true_type
    {
    };
}
