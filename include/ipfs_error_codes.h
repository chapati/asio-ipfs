// Error codes returned by IPFS glue code
#pragma once

#define IPFS_SUCCESS                 0
#define IPFS_NODE_EXISTS             1
#define IPFS_PARSE_CONFIG_FAIL       2
#define IPFS_READ_CONFIG_FAILED      3  // failed to read config from IPFS repo
#define IPFS_CREATE_REPO_FAILED      4
#define IPFS_OPEN_REPO_FAILED        5
#define IPFS_PLUGINS_FAILED          6
#define IPFS_API_ACCESS_FAILED       7
#define IPFS_START_NODE_FAILED       8
#define IPFS_MPROME_INJECT_FAILED    9
#define IPFS_FLATFS_FAILED           10   // flatfs plugin failed
#define IPFS_LEVELDS_FAILED          11  // levelds plugin failed
#define IPFS_NO_NODE                 12  // failed to find a node by handle
#define IPFS_RESOLVE_FAILED          13
#define IPFS_ADD_FAILED              14
#define IPFS_CAT_FAILED              15
#define IPFS_READ_FAILED             16
#define IPFS_PUBLISH_FAILED          17
#define IPFS_PIN_FAILED              18
#define IPFS_UNPIN_FAILED            19
#define IPFS_GC_FAILED               20
#define IPFS_API_FAILED              21
#define IPFS_GATEWAY_FAILED          22
#define IPFS_MFSPIN_FAILED           23
#define IPFS_BUILD_ENV_FAILED        24
