if ("${CMAKE_SYSTEM_NAME}" STREQUAL "Windows")
    cmake_minimum_required (VERSION 3.21)
else ()
    cmake_minimum_required (VERSION 3.13)
endif()

project(asio-ipfs)
include(ExternalProject)

#
# Boost
#
set(BOOST_VERSION 1.62)
find_package(Boost ${BOOST_VERSION} REQUIRED COMPONENTS
    coroutine
    system
)

#
# Detect platform & arch. We do not support cross-compilation 
# so we assume that CMAKE_SYSTEM_NAME and CMAKE_HOST_SYSTEM_NAME are the same
# as well as CMAKE_HOST_SYSTEM_PROCESSOR and CMAKE_SYSTEM_PROCESSOR
#
if ("${CMAKE_SYSTEM_NAME}" STREQUAL "Linux")
    set(LINUX TRUE)
    
    if ("${CMAKE_SYSTEM_PROCESSOR}" STREQUAL "x86_64")
        set(LINUX64 TRUE)
        set(X64 TRUE)
    else()
         message(FATAL_ERROR "Unsupported host architecture ${CMAKE_SYSTEM_PROCESSOR}")
    endif()

elseif("${CMAKE_SYSTEM_NAME}" STREQUAL "Windows")
    set(WINDOWS TRUE)
    
    if ("${CMAKE_SYSTEM_PROCESSOR}" STREQUAL "AMD64")
        set(WIN64 TRUE)
        set(X64 TRUE)
    else()
         message(FATAL_ERROR "Unsupported host architecture ${CMAKE_SYSTEM_PROCESSOR}")
    endif()
elseif("${CMAKE_SYSTEM_NAME}" STREQUAL "Darwin")
    set(DARWIN TRUE)
    if ("${CMAKE_SYSTEM_PROCESSOR}" STREQUAL "x86_64")
        set(DARWIN64 TRUE)
        set(X64 TRUE)
    else()
         message(FATAL_ERROR "Unsupported host architecture ${CMAKE_SYSTEM_PROCESSOR}")
    endif()
else()
    message(FATAL_ERROR "Unsupported host platform ${CMAKE_SYSTEM_NAME}")
endif()

#
# Download and install golang
#
set(GOROOT "${CMAKE_CURRENT_BINARY_DIR}/golang")
set(GOPATH "${CMAKE_CURRENT_BINARY_DIR}/go-workspace")

if (LINUX64)
    set(GOSRC       "https://dl.google.com/go/go1.16.10.linux-amd64.tar.gz")
    set(GOSRC_HASH  "414cd18ce1d193769b9e97d2401ad718755ab47816e13b2a1cde203d263b55cf")
    set(GOOS        "linux")
    set(GOARCH      "amd64")
endif()

if (DARWIN64)
    set(GOSRC       "https://dl.google.com/go/go1.16.10.darwin-amd64.tar.gz")
    set(GOSRC_HASH  "895a3fe6d720297ce16272f41c198648da8675bb244ab6d60003265c176b6c48")
    set(GOOS        "darwin")
    set(GOARCH      "amd64")
endif()

if (WIN64)
    set(GOSRC       "https://dl.google.com/go/go1.16.10.windows-amd64.zip")
    set(GOSRC_HASH  "787c9afbcd7446874f79bffd20dc4219c3aeff6731bd03f09058bdd546c7eb87")
    set(GOOS        "windows")
    set(GOARCH      "amd64")
endif()

externalproject_add(golang
    URL ${GOSRC}
    URL_HASH SHA256=${GOSRC_HASH}
    CONFIGURE_COMMAND ""
    BUILD_COMMAND     ""
    INSTALL_COMMAND   ""
    SOURCE_DIR        ${GOROOT}
)

#
# Install tdm-gcc if on windows. Necessary for CGO compilation step.
# Visual studio compilers are not supported by Golang/CGO at the moment.
#
if (WINDOWS)
    set(TDMROOT "${CMAKE_CURRENT_BINARY_DIR}/tdmgcc")
    make_directory(${TDMROOT})

    externalproject_add(tdmgcca
        URL https://github.com/jmeubank/tdm-gcc-src/releases/download/v10.3.0-tdm64-1/gcc-10.3.0-tdm64-1-core.zip
        URL_HASH SHA256=6514c8c2e562d37f3174bad65f0e2498246c60b85a166bf36b124da55a1709fc
        CONFIGURE_COMMAND ""
        BUILD_COMMAND ""
        INSTALL_COMMAND ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_BINARY_DIR}/tdmgcca-src ${TDMROOT}
        SOURCE_DIR ${CMAKE_CURRENT_BINARY_DIR}/tdmgcca-src
    )

    externalproject_add(tdmgccb
        URL https://github.com/jmeubank/tdm-binutils-gdb/releases/download/v2.36.1-tdm64-1/binutils-2.36.1-tdm64-1.zip
        URL_HASH SHA256=e3e6ca2fd86874c25b5e69e85862b35489c0f381d14bec450274c902642ff22e
        CONFIGURE_COMMAND ""
        BUILD_COMMAND ""
        INSTALL_COMMAND ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_BINARY_DIR}/tdmgccb-src ${TDMROOT}
        SOURCE_DIR ${CMAKE_CURRENT_BINARY_DIR}/tdmgccb-src
        DEPENDS tdmgcca
    )

    externalproject_add(tdmgccc
        URL https://github.com/jmeubank/mingw-w64/releases/download/v8-git2021050601-gcc10-tdm64-1/mingw64runtime-v8-git2021050601-gcc10-tdm64-1.zip
        URL_HASH SHA256=2f9c7c79f51d968db8659b19dff6778fa504cb482315a2e950922ccb29da6e85
        CONFIGURE_COMMAND ""
        BUILD_COMMAND ""
        INSTALL_COMMAND ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_BINARY_DIR}/tdmgccc-src ${TDMROOT}
        SOURCE_DIR ${CMAKE_CURRENT_BINARY_DIR}/tdmgccc-src
        DEPENDS tdmgccb
    )

    externalproject_add(tdmgccd
        URL https://github.com/jmeubank/tdm-gcc-src/releases/download/v10.3.0-tdm64-1/gcc-10.3.0-tdm64-1-c++.zip
        URL_HASH SHA256=36d48e2a7697b90bf1375fd16b45f3a802fadab5c677b5dd63357fb1ee25187a
        CONFIGURE_COMMAND ""
        BUILD_COMMAND ""
        INSTALL_COMMAND ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_BINARY_DIR}/tdmgccd-src ${TDMROOT}
        SOURCE_DIR ${CMAKE_CURRENT_BINARY_DIR}/tdmgccd-src
        DEPENDS tdmgccc
    )

    externalproject_add(tdmgcce
        URL https://github.com/jmeubank/windows-default-manifest/releases/download/v6.4-x86_64_multi/windows-default-manifest-6.4-x86_64_multi.zip
        URL_HASH SHA256=d34b4d149242500cba357ec80fc729dd30f40e120ee022aee7d7af4e9da3a89f
        CONFIGURE_COMMAND ""
        BUILD_COMMAND ""
        INSTALL_COMMAND ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_BINARY_DIR}/tdmgcce-src ${TDMROOT}
        SOURCE_DIR ${CMAKE_CURRENT_BINARY_DIR}/tdmgcce-src
        DEPENDS tdmgccd
    )   
endif()

#
# Build libipfs-bindings. This is a wrapper around go-ipfs
# that produces a shared library [.dll + .lib + .h] on Windows
# and a static library [.a + .h] on other operating systems.
#
# Since CGO uses GCC and Beam itself uses MSVC the only way
# on Windows is to use a DLL. On other operating systems the
# story is much easier and we can use usual lib/.a and not
# overcomplicate our build & install process
#
set(BINDINGS_DIR     "${CMAKE_CURRENT_BINARY_DIR}/ipfs_bindings")
set(GOPATH_IPFS_DIR  "${GOPATH}/src/github.com/equalitie/asio-ipfs")

set(BINDINGS_SRC
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_bindings.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_node.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_config.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_cancel.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_repo.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_gc.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_api.go
    ${CMAKE_CURRENT_SOURCE_DIR}/src/ipfs_bindings/ipfs_mfs.go
)

if (LINUX)
    set(BINDINGS_HEADER  "${BINDINGS_DIR}/libipfs-bindings.h")
    set(BINDINGS_LIBRARY "${BINDINGS_DIR}/libipfs-bindings.a")
    set(BINDINGS_OUTPUT  ${BINDINGS_HEADER} ${BINDINGS_LIBRARY})

    add_custom_command(
        OUTPUT  ${BINDINGS_OUTPUT}
        DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/include/ipfs_error_codes.h
                ${BINDINGS_SRC}
                golang

        COMMAND ${CMAKE_COMMAND} -E make_directory ${BINDINGS_DIR}
            && ${CMAKE_COMMAND} -E make_directory ${GOPATH_IPFS_DIR}
            && ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_SOURCE_DIR}/include ${GOPATH_IPFS_DIR}/include
            && ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_SOURCE_DIR}/src ${GOPATH_IPFS_DIR}/src
            && ${CMAKE_COMMAND} -E copy ${CMAKE_CURRENT_SOURCE_DIR}/go.mod ${GOPATH_IPFS_DIR}
            && ${CMAKE_COMMAND} -E copy ${CMAKE_CURRENT_SOURCE_DIR}/go.sum ${GOPATH_IPFS_DIR}

        COMMAND cd ${GOPATH_IPFS_DIR}
             && export GOROOT=${GOROOT}
             && export GOPATH=${GOPATH}
             && export GOOS=${GOOS}
             && export GOARCH=${GOARCH}
             && export GOARM=${GOARM}
             && export CGO_ENABLED=1
             && export CC=${CMAKE_C_COMPILER}
             && export GO111MODULE=on
             && export GOGCCFLAGS="-m64 -mthreads -fno-caret-diagnostics -Qunused-arguments -fmessage-length=0 -gno-record-gcc-switches"
             && ${GOROOT}/bin/go build
                    --buildmode=c-archive
                    -ldflags "-s -w"
                    -modcacherw
                    -o ${BINDINGS_LIBRARY}
                    ./src/ipfs_bindings
    )

elseif (DARWIN)
    set(BINDINGS_HEADER  "${BINDINGS_DIR}/libipfs-bindings.h")
    set(BINDINGS_LIBRARY "${BINDINGS_DIR}/libipfs-bindings.a")
    set(BINDINGS_OUTPUT  ${BINDINGS_HEADER} ${BINDINGS_LIBRARY})

    add_custom_command(
        OUTPUT  ${BINDINGS_OUTPUT}
        DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/include/ipfs_error_codes.h
                ${BINDINGS_SRC}
                golang

        COMMAND ${CMAKE_COMMAND} -E make_directory ${BINDINGS_DIR}
            && ${CMAKE_COMMAND} -E make_directory ${GOPATH_IPFS_DIR}
            && ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_SOURCE_DIR}/include ${GOPATH_IPFS_DIR}/include
            && ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_SOURCE_DIR}/src ${GOPATH_IPFS_DIR}/src
            && ${CMAKE_COMMAND} -E copy ${CMAKE_CURRENT_SOURCE_DIR}/go.mod ${GOPATH_IPFS_DIR}
            && ${CMAKE_COMMAND} -E copy ${CMAKE_CURRENT_SOURCE_DIR}/go.sum ${GOPATH_IPFS_DIR}

        COMMAND cd ${GOPATH_IPFS_DIR}
             && export GOROOT=${GOROOT}
             && export GOPATH=${GOPATH}
             && export GOOS=${GOOS}
             && export GOARCH=${GOARCH}
             && export GOARM=${GOARM}
             && export CGO_ENABLED=1
             && export CC=clang
             && export GO111MODULE=on
             && export GOGCCFLAGS="-m64 -fno-caret-diagnostics -Qunused-arguments -fmessage-length=0 -gno-record-gcc-switches"
             && ${GOROOT}/bin/go build
                    --buildmode=c-archive
                    -ldflags "-s -w"
                    -modcacherw
                    -o ${BINDINGS_LIBRARY}
                    ./src/ipfs_bindings
    )
elseif(WIN64)
    set(BINDINGS_HEADER  "${BINDINGS_DIR}/ipfs-bindings.h")
    set(BINDINGS_LIBRARY "${BINDINGS_DIR}/ipfs-bindings.lib")
    set(BINDINGS_DLL     "${BINDINGS_DIR}/ipfs-bindings.dll")
    set(BINDINGS_DEF     "${BINDINGS_DIR}/ipfs-bindings.def")
    set(BINDINGS_OUTPUT  ${BINDINGS_HEADER} ${BINDINGS_DLL} ${BINDINGS_LIBRARY})

    add_custom_command(
        OUTPUT ${BINDINGS_OUTPUT}       
        DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/include/ipfs_error_codes.h
                ${BINDINGS_SRC}
                golang tdmgcce

        COMMAND ${CMAKE_COMMAND} -E make_directory ${BINDINGS_DIR}        
             && ${CMAKE_COMMAND} -E make_directory ${GOPATH_IPFS_DIR}
             && ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_SOURCE_DIR}/include ${GOPATH_IPFS_DIR}/include
             && ${CMAKE_COMMAND} -E copy_directory ${CMAKE_CURRENT_SOURCE_DIR}/src ${GOPATH_IPFS_DIR}/src
             && ${CMAKE_COMMAND} -E copy ${CMAKE_CURRENT_SOURCE_DIR}/go.mod ${GOPATH_IPFS_DIR}
             && ${CMAKE_COMMAND} -E copy ${CMAKE_CURRENT_SOURCE_DIR}/go.sum ${GOPATH_IPFS_DIR}

        COMMAND cd ${GOPATH_IPFS_DIR}       
             && ${CMAKE_COMMAND} -E env 
                "GOROOT=${GOROOT}" 
                "GOPATH=${GOPATH}"
                "GOOS=${GOOS}"
                "GOARCH=${GOARCH}"
                "CGO_ENABLED=1"
                "CC=${TDMROOT}/bin/gcc.exe"
                "GO111MODULE=on"
                "GOTESTMSVC=1"
                "GOGCCFLAGS=-m64 -mthreads -fno-caret-diagnostics -Qunused-arguments -fmessage-length=0 -gno-record-gcc-switches"
                ${GOROOT}/bin/go build
                    --buildmode=c-shared
                    -ldflags "-s -w"
                    -o ${BINDINGS_DLL}
                    -modcacherw
                    ./src/ipfs_bindings
        COMMAND cd ${BINDINGS_DIR}
             && ${TDMROOT}/x86_64-w64-mingw32/bin/gendef.exe ipfs-bindings.dll
             && lib /def:${BINDINGS_DEF} /machine:x64 
    )
endif()

add_custom_target(ipfs-bindings-build ALL
    DEPENDS ${BINDINGS_OUTPUT}
)

if (BEAM_SIGN_PACKAGE AND WINDOWS)
    add_custom_command(
        TARGET ipfs-bindings-build POST_BUILD
        COMMAND ${SIGNTOOL_EXECUTABLE} sign ${SIGNTOOL_OPTIONS} ${BINDINGS_DLL}
    )
endif()

if (WINDOWS)
    add_library(ipfs-bindings SHARED IMPORTED GLOBAL
        DEPENDS ipfs-bindings-build 
    )

    set_target_properties(ipfs-bindings PROPERTIES
        IMPORTED_LOCATION ${BINDINGS_DLL}
        IMPORTED_IMPLIB ${BINDINGS_LIBRARY}
    )

    # IMPORTED_RUNTIME_ARTIFACTS available only in 3.21
    install(IMPORTED_RUNTIME_ARTIFACTS ipfs-bindings RUNTIME DESTINATION .)
    # install(FILES ${BINDINGS_DLL} DESTINATION BIN)
endif()

if (LINUX OR DARWIN)
    add_library(ipfs-bindings STATIC IMPORTED GLOBAL
        DEPENDS ipfs-bindings-build
    )

    set_property(TARGET ipfs-bindings PROPERTY
        IMPORTED_LOCATION ${BINDINGS_LIBRARY}
    )
endif()

if (DARWIN)
    target_link_libraries(ipfs-bindings
        INTERFACE
            "-framework CoreFoundation"
            "-framework Security")
endif()

#
# asio-ipfs itself
#
file(GLOB sources "./src/*.cpp")

add_library(asio-ipfs 
    STATIC ${sources}
)

set_target_properties(asio-ipfs
    PROPERTIES LINKER_LANGUAGE CXX
)

if (WINDOWS)
    set_target_properties(asio-ipfs
        PROPERTIES IPFS_DLL ${BINDINGS_DLL}
    )
endif()

add_dependencies(asio-ipfs 
    ipfs-bindings
    ipfs-bindings-build
)

target_include_directories(asio-ipfs
    PRIVATE ${BINDINGS_DIR}
    PRIVAte "${CMAKE_CURRENT_SOURCE_DIR}/include"
)

target_link_libraries(asio-ipfs
    PUBLIC ipfs-bindings
    PUBLIC Boost::boost
    INTERFACE ${Boost_LIBRARIES}
)
