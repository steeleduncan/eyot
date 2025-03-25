#include <unistd.h>
#include <stdio.h>
#include "eyot-runtime-common.h"

void ffi_os_usleep(EyExecutionContext *ctx, EyInteger count) {
    usleep(count);
}

void ffi_os_exit(EyExecutionContext *ctx, EyInteger code) {
    exit(code);
}
