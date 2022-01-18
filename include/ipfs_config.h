#pragma once
#include <string>
#include <vector>

namespace asio_ipfs
{
    // TODO:IPFS consider supporting lowpower profile via UI options
    struct config {
        enum class Mode {
            Desktop,
            Server,
        };

        explicit config (enum Mode mode) {
            if (mode == Mode::Server) {
                auto_relay = false;
                default_profile = "server";
                storage_max = "20GB";
            }
        }

        //
        // N.B. Defaults below are optimal for running IPFS on desktop
        //
        bool online = true;
        std::string repo_root = "./ipfs-repo";
        std::string default_profile;
        std::string storage_max = "2GB";
        unsigned int low_water = 100;
        unsigned int high_water = 200;
        unsigned int grace_period = 20;
        unsigned int node_swarm_port = 10100;
        unsigned int node_api_port = 6001; // TODO:IPFS disable on desktop
        std::vector<std::string> bootstrap;
        bool auto_relay = true;
        bool relay_hop = false;
    };
}
