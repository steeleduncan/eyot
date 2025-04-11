/*
  Eyot entrypoint code
 */

#include "eyot-runtime-cpu.h"

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <stdarg.h>

void ey_print(const char *msg, ...) {
    va_list ap;
    va_start(ap, msg);
    vfprintf(stdout, msg, ap);
    va_end(ap);
}

void ey_print_byte(EyExecutionContext *ctx __attribute__((unused)), char val) {
    fputc(val, stdout);
}

void ey_runtime_panic(const char *unit, const char *msg) {
    fprintf(stderr, "%s: %s\n", unit, msg);
    exit(1);
}

static EyGCRegion *global_gc = 0;

EyGCRegion *ey_runtime_gc(EyExecutionContext *ctx __attribute__((unused))) {
    return global_gc;
}

void ey_runtime_collect(EyExecutionContext *ctx) {
    return ey_runtime_gc_collect(ey_runtime_gc(ctx));
}

EyInteger ey_runtime_allocated_bytes(EyExecutionContext *ctx) {
    return ey_runtime_gc_get_stats(ey_runtime_gc(ctx)).bytes_allocated;
}

static EyVector *args_vector = 0;

EyVector *ey_runtime_get_args(EyExecutionContext *ctx __attribute__((unused))) {
    return args_vector;
}

int main(int argc, const char **argv) {
    global_gc = ey_runtime_gc_create();

#if defined(EYOT_OPENCL_INCLUDED)
    if (ey_runtime_cl_src) {
        ey_init_opencl(ey_runtime_cl_src);
    }
#endif  // EYOT_OPENCL_INCLUDED

    EyExecutionContext ctx = {};

    args_vector = ey_vector_create(&ctx, sizeof(EyString));
    for (int i = 0; i < argc; i += 1) {
        EyString arg = ey_runtime_string_create_literal(&ctx, argv[i]);
        ey_vector_append(&ctx, args_vector, &arg);
    }
    ey_runtime_gc_remember_root_object(global_gc, args_vector);

    ey_generated_main(&ctx);

    ey_runtime_gc_forget_root_object(global_gc, args_vector);
    ey_runtime_gc_free(global_gc);
    return 0;
}
