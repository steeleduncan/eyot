// currently part of eyot-runtime
#include <stdlib.h>
#include "eyot-runtime-common.h"

void ffi_panic(EyExecutionContext *ctx) {
    ey_runtime_panic("runtime", "panic");
}
