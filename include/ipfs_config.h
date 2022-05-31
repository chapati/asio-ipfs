#pragma once
#include <string>
#include <vector>

namespace asio_ipfs
{
    struct config {
        enum class Mode {
            Desktop,
            Server,
        };

        config (enum Mode mode, const std::string& skey)
            : swarm_key(skey)
        {
            if (mode == Mode::Server) {
                auto_relay = false;
                default_profile = "server";
                storage_max = "20GB";
                api_address = "/ip4/127.0.0.1/tcp/6100";;
                gateway_address = "/ip4/127.0.0.1/tcp/6200";
                routing_type = "dht";
                run_gc = false;
            }
        }

        //
        // N.B. Defaults below are optimal for running IPFS on desktop
        //
        std::string repo_root = "./ipfs-repo";
        std::string default_profile;
        std::string storage_max = "2GB";
        unsigned int low_water = 100;
        unsigned int high_water = 200;
        unsigned int grace_period = 20;
        unsigned int swarm_port = 10100;
        std::string gateway_address;
        std::string api_address;
        bool autonat = true;
        unsigned int autonat_limit = 30;
        unsigned int autonat_peer_limit = 3;
        std::vector<std::string> bootstrap;
        std::vector<std::string> peering;
        std::string swarm_key;
        std::string routing_type = "dht";
        bool auto_relay = true;
        bool relay_hop = false;
        bool run_gc = true;
    };
}
