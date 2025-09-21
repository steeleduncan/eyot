#include <stdlib.h>
#include "eyot-runtime-cpu.h"

void ffi_panic(EyExecutionContext *ctx) {
    ey_runtime_panic("runtime", "panic");
}
