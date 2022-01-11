// Error codes returned by IPFS glue code
#pragma once

#ifndef GUARD_ipfs_error_codes_h
#define GUARD_ipfs_error_codes_h

#define IPFS_SUCCESS                 0
#define IPFS_FAILED_TO_PARSE_CONFIG  1
#define IPFS_FAILED_TO_CREATE_REPO   2  // failed to create repository
#define IPFS_FAILED_TO_START_NODE    3  // failed to start node
#define IPFS_RESOLVE_FAILED          4  // failed to resolve IPNS entry
#define IPFS_ADD_FAILED              5  // failed to add data
#define IPFS_CAT_FAILED              6  // failed to get data reader
#define IPFS_READ_FAILED             7  // failed to read data
#define IPFS_PUBLISH_FAILED          8  // failed to publish CID
#define IPFS_PIN_FAILED              9  // failed to publish CID
#define IPFS_UNPIN_FAILED           10  // failed to unpin CID
#define IPFS_GC_FAILED              11  // failed to garbage collect
#define IPFS_NO_NODE                12  // failed to find a node by handle


#endif  // ndef GUARD_ipfs_error_codes_h
