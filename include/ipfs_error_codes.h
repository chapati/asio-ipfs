// Error codes returned by IPFS glue code
#pragma once

#define IPFS_SUCCESS                 0
#define IPFS_PARSE_CONFIG_FAIL       1
#define IPFS_READ_CONFIG_FAILED      2  // failed to read config from IPFS repo
#define IPFS_CREATE_REPO_FAILED      3
#define IPFS_CREATE_API_FAILED       4
#define IPFS_START_NODE_FAILED       5
#define IPFS_READ_PEERS_FAILED       6
#define IPFS_MPROME_INJECT_FAILED    7
#define IPFS_FLATFS_FAILED           8  // flatfs plugin failed
#define IPFS_LEVELDS_FAILED          9  // levelds plugin failed
#define IPFS_NO_NODE                 10  // failed to find a node by handle
#define IPFS_RESOLVE_FAILED          11
#define IPFS_ADD_FAILED              12
#define IPFS_CAT_FAILED              13
#define IPFS_READ_FAILED             14
#define IPFS_PUBLISH_FAILED          15
#define IPFS_PIN_FAILED              16
#define IPFS_UNPIN_FAILED            17
#define IPFS_GC_FAILED               18
