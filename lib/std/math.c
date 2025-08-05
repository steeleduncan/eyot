#include "eyot-runtime-cpu.h"
#include <math.h>

EyFloat64 ey_stdlib_sqrtd(EyExecutionContext *ctx, EyFloat64 val) {
    return sqrt(val);
}

EyFloat32 ey_stdlib_sqrtf(EyExecutionContext *ctx, EyFloat32 val) {
    return sqrtf(val);
}
