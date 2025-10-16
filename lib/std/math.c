#include "eyot-runtime-cpu.h"
#include <math.h>
#include <stdlib.h>

EyFloat64 ey_stdlib_sqrtd(EyExecutionContext *ctx, EyFloat64 val) {
    return sqrt(val);
}

EyFloat64 ey_stdlib_expd(EyExecutionContext *ctx, EyFloat64 val) {
    return exp(val);
}

EyFloat64 ey_stdlib_logd(EyExecutionContext *ctx, EyFloat64 val) {
    return log(val);
}

EyFloat64 ey_stdlib_cosd(EyExecutionContext *ctx, EyFloat64 val) {
    return cos(val);
}

EyFloat64 ey_stdlib_sind(EyExecutionContext *ctx, EyFloat64 val) {
    return sin(val);
}

EyFloat64 ey_stdlib_tand(EyExecutionContext *ctx, EyFloat64 val) {
    return tan(val);
}

EyFloat32 ey_stdlib_sqrtf(EyExecutionContext *ctx, EyFloat32 val) {
    return sqrtf(val);
}

EyInteger ey_stdlib_rand(EyExecutionContext *ctx) {
    return rand();
}

EyInteger ey_stdlib_rand_max(EyExecutionContext *ctx) {
    return RAND_MAX;
}
