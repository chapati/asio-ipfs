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

        explicit config (enum Mode mode) {
            if (mode == Mode::Server) {
                auto_relay = false;
                default_profile = "server";
                storage_max = "20GB";
                api_port = 6100;
                gateway_port = 6200;
                routing_type = "dhtserver";
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
        unsigned int gateway_port = 0;
        unsigned int api_port = 0;
        bool autonat = true;
        unsigned int autonat_limit = 30;
        unsigned int autonat_peer_limit = 3;
        std::vector<std::string> bootstrap;
        std::vector<std::string> peering;
        std::string swarm_key = "/key/swarm/psk/1.0.0/\n/base16/\n1191aea7c9f99f679f477411d9d44f1ea0fdf5b42d995966b14a9000432f8c4a";
        std::string routing_type = "dht";
        bool auto_relay = true;
        bool relay_hop = false;
        bool run_gc = true;
    };
}
