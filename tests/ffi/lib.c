#include "eyot-runtime-common.h"

static int saved_var = 0;

void ffi_set_var(EyExecutionContext *ctx, EyInteger var) {
    saved_var = var;
}

EyInteger ffi_get_var(EyExecutionContext *ctx) {
    return saved_var;
}
