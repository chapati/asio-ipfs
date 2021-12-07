#pragma once

#include <string>
#include <vector>

namespace asio_ipfs
{
    struct config {
        bool         online       = true;
        unsigned int low_water    = 600; // TODO:IPFS what is this?
        unsigned int high_water   = 900; // TODO:IPFS what is this?
        unsigned int grace_period = 20;  // seconds // TODO:IPFS what is this?
        std::vector<std::string> bootstrap;
    };
}
