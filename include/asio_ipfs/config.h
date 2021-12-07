#pragma once

#include <string>
#include <vector>

namespace asio_ipfs
{
    struct config {
        bool         online       = true;
        unsigned int low_water    = 100;
        unsigned int high_water   = 200;
        unsigned int grace_period = 20;
        std::vector<std::string> bootstrap;
    };
}
