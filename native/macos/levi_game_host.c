/* SPDX-License-Identifier: GPL-3.0-only
 *
 * LeviGameHost: minimal macOS x86_64 Game Mode host wrapper.
 *
 * Launched through LaunchServices (`open -n -a LeviGameHost.app`) so macOS
 * RunningBoard attributes the game process to a bundle declaring
 * LSSupportsGameMode=YES, then sets DYLD_INSERT_LIBRARIES from the inert
 * LEVI_METAL_HELPER_LIB env var and execv's MetalSharp's inner
 * `lib/wine/x86_64-unix/wine` loader (which retains Wine's Mach-O address-space
 * segments and disable-library-validation entitlement without modifying or
 * re-signing MetalSharp-Wine-x86_64.app).
 */
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "function_keys.h"

static int prepend_dyld_library(const char *lib_path) {
    if (!lib_path || lib_path[0] != '/') {
        return -1;
    }
    if (access(lib_path, R_OK) != 0) {
        fprintf(stderr, "LeviGameHost: Metal helper unreadable: %s (%s)\n", lib_path, strerror(errno));
        return -1;
    }
    const char *existing = getenv("DYLD_INSERT_LIBRARIES");
    if (!existing || existing[0] == '\0') {
        return setenv("DYLD_INSERT_LIBRARIES", lib_path, 1);
    }
    size_t lib_len = strlen(lib_path);
    const char *cursor = existing;
    while (*cursor) {
        const char *colon = strchr(cursor, ':');
        size_t part_len = colon ? (size_t)(colon - cursor) : strlen(cursor);
        if (part_len == lib_len && memcmp(cursor, lib_path, lib_len) == 0) {
            return 0;
        }
        if (!colon) break;
        cursor = colon + 1;
    }
    size_t existing_len = strlen(existing);
    char *combined = (char *)malloc(lib_len + 1 + existing_len + 1);
    if (!combined) return -1;
    memcpy(combined, lib_path, lib_len);
    combined[lib_len] = ':';
    memcpy(combined + lib_len + 1, existing, existing_len + 1);
    int rc = setenv("DYLD_INSERT_LIBRARIES", combined, 1);
    free(combined);
    return rc;
}

int main(int argc, char **argv) {
    if (argc == 3 && strcmp(argv[1], "--restore-function-keys") == 0) {
        // The game sends a byte after a normal restore. EOF also covers a crash.
        char restored;
        ssize_t result;
        do { result = read(STDIN_FILENO, &restored, 1); } while (result < 0 && errno == EINTR);
        if (result != 0) return result < 0;
        io_connect_t connection = levi_open_keyboard_parameters();
        if (!connection) return 1;
        bool ok = levi_set_function_key_mode(connection, atoi(argv[2]));
        IOServiceClose(connection);
        return ok ? 0 : 1;
    }

    const char *wine_env = getenv("LEVI_WINE_UNIX_BIN");
    if (!wine_env || wine_env[0] != '/') {
        fprintf(stderr, "LeviGameHost: missing absolute LEVI_WINE_UNIX_BIN\n");
        return 126;
    }
    char *wine_bin = strdup(wine_env);
    if (!wine_bin) {
        fprintf(stderr, "LeviGameHost: out of memory\n");
        return 126;
    }
    if (access(wine_bin, X_OK) != 0) {
        fprintf(stderr, "LeviGameHost: Wine loader is not executable: %s (%s)\n", wine_bin, strerror(errno));
        free(wine_bin);
        return 126;
    }

    const char *helper_env = getenv("LEVI_METAL_HELPER_LIB");
    if (helper_env && helper_env[0] != '\0') {
        if (prepend_dyld_library(helper_env) != 0) {
            fprintf(stderr, "LeviGameHost: continuing without Metal helper\n");
        }
    }
    unsetenv("LEVI_METAL_HELPER_LIB");
    unsetenv("LEVI_WINE_UNIX_BIN");

    if (argv[0][0] == '/') {
        setenv("LEVI_GAME_HOST_BIN", argv[0], 1);
    }

    char **new_argv = (char **)calloc((size_t)argc + 2, sizeof(char *));
    if (!new_argv) {
        fprintf(stderr, "LeviGameHost: out of memory\n");
        free(wine_bin);
        return 126;
    }
    int dst = 0;
    new_argv[dst++] = wine_bin;
    for (int i = 1; i < argc; ++i) {
        /* macOS LaunchServices may pass a process serial number argument. */
        if (i == 1 && strncmp(argv[i], "-psn_", 5) == 0) {
            continue;
        }
        new_argv[dst++] = argv[i];
    }
    new_argv[dst] = NULL;

    execv(wine_bin, new_argv);
    fprintf(stderr, "LeviGameHost: execv(%s) failed: %s\n", wine_bin, strerror(errno));
    free(new_argv);
    free(wine_bin);
    return 127;
}
