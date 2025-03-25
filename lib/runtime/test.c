#include "eyot-runtime-cpu.h"
#include <stdlib.h>
#include <stdio.h>
#include <pthread.h>
#include <string.h>

/*
  Shims (provided by generated eyot)
*/
const char *ey_runtime_cl_src = "";

EyStringS *ey_string_pool_raw = 0;

int ey_generated_arg_count(int fid __attribute__((unused))) {
    return 3;
}

void ey_functioncaller(EyExecutionContext *ctx __attribute__((unused)),
                       int fid __attribute__((unused)), void *result __attribute__((unused)),
                       void **args __attribute__((unused))) {
}

int ey_generated_closure_arg_size(int fid __attribute__((unused)),
                                  int argument __attribute__((unused))) {
    return sizeof(int);
}
/*
  Test code
 */

static void test_vector(EyExecutionContext *ctx) {
    EyVector *v = ey_vector_create(ctx, sizeof(int));

    for (int i = 0; i < 5; i += 1) {
        ey_vector_append(ctx, v, &i);
    }

    if (ey_vector_length(ctx, v) != 5) {
        ey_runtime_panic("test_vector", "A");
    }

    const int after[] = {0, 3, 4};
    ey_vector_erase(ctx, v, 1, 2);

    if (ey_vector_length(ctx, v) != 3) {
        ey_runtime_panic("test_vector", "B");
    }

    for (int i = 0; i < 3; i += 1) {
        const int kv = *(int *)ey_vector_access(ctx, v, i);
        if (kv != after[i]) {
            ey_runtime_panic("test_vector", "C");
        }
    }
}

static int ival = 0;
void wrkr(EyExecutionContext *ectx __attribute__((unused)), void *in,
          void *out __attribute__((unused)), void *ctx) {
    int *val = in;
    ival += *val;

    int *ctxi = ctx;

    // it can perform inner mutations, is that a good thing?
    if (!(*ctxi == 1234 || *ctxi == 1235)) {
        ey_runtime_panic("test_basic_worker", "bad value in");
    }
    *ctxi += 1;
}

/*
  Test
  - void worker
  - non-nil context
 */
static void test_basic_worker(EyExecutionContext *ctx) {
    int ictx = 1234;

    EyWorker *w = ey_worker_create_cpu(wrkr, sizeof(int), 0, &ictx, sizeof(ictx));
    EyVector *values = ey_vector_create(ctx, sizeof(int));

    int v = 1;
    ey_vector_append(ctx, values, &v);
    v = 2;
    ey_vector_append(ctx, values, &v);

    w->send(w, values);
    w->drain(w);

    if (ival != 3) {
        ey_runtime_panic("test_basic_worker", "bad value");
    }

    if (ictx != 1234) {
        ey_runtime_panic("test_basic_worker", "bad ctx out");
    }
}

void double_worker(EyExecutionContext *ectx __attribute__((unused)), void *in, void *out,
                   void *ctx __attribute__((unused))) {
    int *cast_in = in;
    int *cast_out = out;

    *cast_out = *cast_in * 2;
}

void increment_worker(EyExecutionContext *ectx __attribute__((unused)), void *in, void *out,
                      void *ctx __attribute__((unused))) {
    int *cast_in = in;
    int *cast_out = out;

    *cast_out = *cast_in + 1;
}

static void test_strings(EyExecutionContext *ctx) {
    EyString s = ey_runtime_string_create_literal(ctx, "hello"),
             t = ey_runtime_string_create_literal(ctx, " there"),
             u = ey_runtime_string_join(ctx, s, t),
             v = ey_runtime_string_create_literal(ctx, "hello there");

    if (ey_runtime_string_character_length(ctx, s) != 5) {
        ey_runtime_panic("test_strings", "wrong length 1");
    }

    if (ey_runtime_string_character_length(ctx, t) != 6) {
        ey_runtime_panic("test_strings", "wrong length 2");
    }

    if (ey_runtime_string_character_length(ctx, u) != 11) {
        ey_runtime_panic("test_strings", "wrong length 3");
    }

    if (!ey_runtime_string_equality(ctx, u, v)) {
        ey_runtime_panic("test_strings", "wrong appended string");
    }
}

/*
  Test
  - returns from worker
  - null context passed in
 */
static void test_returning_worker(EyExecutionContext *ctx) {
    EyWorker *w = ey_worker_create_cpu(increment_worker, sizeof(int), sizeof(int), 0, 0);

    EyVector *values = ey_vector_create(ctx, sizeof(int));

    int v = 1;
    ey_vector_append(ctx, values, &v);
    v = 2;
    ey_vector_append(ctx, values, &v);
    v = 3;
    ey_vector_append(ctx, values, &v);

    w->send(w, values);

    int r;
    w->receive(w, &r);
    if (r != 2) {
        ey_runtime_panic("test", "bad value 1");
    }

    EyVector *vec = w->drain(w);
    if (ey_vector_length(ctx, vec) != 2) {
        printf("len = %i\n", ey_vector_length(ctx, vec));
        ey_runtime_panic("test", "bad vector length");
    }

    r = *(int *)ey_vector_access(ctx, vec, 0);
    if (r != 3) {
        ey_runtime_panic("test", "bad value 2");
    }

    r = *(int *)ey_vector_access(ctx, vec, 1);
    if (r != 4) {
        ey_runtime_panic("test", "bad value 3");
    }
}

static void test_pipeline(EyExecutionContext *ctx) {
    return;
    EyWorker *double_w = ey_worker_create_cpu(double_worker, sizeof(int), sizeof(int), 0, 0);
    EyWorker *increment_w = ey_worker_create_cpu(increment_worker, sizeof(int), sizeof(int), 0, 0);

    EyWorker *combined = ey_worker_create_pipeline(double_w, increment_w);

    EyVector *values = ey_vector_create(ctx, sizeof(int));

    int v = 1;
    ey_vector_append(ctx, values, &v);
    v = 2;
    ey_vector_append(ctx, values, &v);
    v = 3;
    ey_vector_append(ctx, values, &v);

    combined->send(combined, values);

    int r;
    combined->receive(combined, &r);
    if (r != 3) {
        printf("r = %i\n", r);
        ey_runtime_panic("test", "bad value 1");
    }

    EyVector *vec = combined->drain(combined);
    if (ey_vector_length(ctx, vec) != 2) {
        ey_runtime_panic("test", "bad vector length");
    }

    r = *(int *)ey_vector_access(ctx, vec, 0);
    if (r != 5) {
        ey_runtime_panic("test", "bad value 2");
    }

    r = *(int *)ey_vector_access(ctx, vec, 1);
    if (r != 7) {
        ey_runtime_panic("test", "bad value 3");
    }
}

static const char *cl_src =
    "__kernel void kernel1(__global float* input, __global float* output, const unsigned int "
    "count, __global void * shared) {\n"
    "   int i = get_global_id(0);\n"
    "   if (i < count) {\n"
    "       output[i] = input[i] * input[i];\n"
    "   }\n"
    "}\n"
    "__kernel void kernel2(__global float* input, __global float* output, const unsigned int "
    "count, __global void * shared) {\n"
    "   int i = get_global_id(0);\n"
    "   if (i < count) {\n"
    "       output[i] = input[i] * 2.0;\n"
    "   }\n"
    "}\n"
    "__kernel void kernel3(__global float* input, __global float* output, const unsigned int "
    "count, __global void * shared, __global int * fake_closure) {\n"
    "   int i = get_global_id(0);\n"
    "   if (i < count) {\n"
    "       output[i] = input[i] * (float)*fake_closure;\n"
    "   }\n"
    "}\n";

static void test_gpu_worker(EyExecutionContext *ctx) {
    EyWorker *w = ey_worker_create_opencl("kernel1", 4, 4, 0, 0);
    if (!w) {
        ey_runtime_panic("test", "no worker");
    }

    EyVector *values = ey_vector_create(ctx, sizeof(float));

    float v = 1;
    ey_vector_append(ctx, values, &v);
    v = 2;
    ey_vector_append(ctx, values, &v);
    v = 3;
    ey_vector_append(ctx, values, &v);

    w->send(w, values);
    w->send(w, values);

    float vv;
    w->receive(w, &vv);
    if (vv != 1) {
        ey_runtime_panic("test", "bad receive 1");
    }
    w->receive(w, &vv);
    if (vv != 4) {
        ey_runtime_panic("test", "bad receive 2");
    }

    EyVector *return_values = w->drain(w);
    if (ey_vector_length(ctx, return_values) != 4) {
        ey_runtime_panic("test", "wrong number of return values");
    }
    vv = *(float *)ey_vector_access(ctx, return_values, 0);
    if (vv != 9) {
        ey_runtime_panic("test", "bad val 0");
    }
    vv = *(float *)ey_vector_access(ctx, return_values, 1);
    if (vv != 1) {
        ey_runtime_panic("test", "bad val 1");
    }
    vv = *(float *)ey_vector_access(ctx, return_values, 2);
    if (vv != 4) {
        ey_runtime_panic("test", "bad val 2");
    }
    vv = *(float *)ey_vector_access(ctx, return_values, 3);
    if (vv != 9) {
        ey_runtime_panic("test", "bad val 3");
    }
}

static void test_gpu_worker_with_parameter(EyExecutionContext *ctx) {
    int closure = 2;

    EyWorker *w = ey_worker_create_opencl("kernel3", 4, 4, &closure, sizeof(closure));
    if (!w) {
        ey_runtime_panic("test", "no worker");
    }

    EyVector *values = ey_vector_create(ctx, sizeof(float));

    float v = 2;
    ey_vector_append(ctx, values, &v);

    w->send(w, values);

    float vv;
    w->receive(w, &vv);
    if (vv != 4) {
        ey_runtime_panic("test", "bad receive 1");
    }
}

static int finalised = 0;

static void finaliser(void *ptr) {
    const int val = *(int *)ptr;

    if (finalised & val) {
        ey_runtime_panic("test", "re-finalising");
    }
    finalised |= val;
}

static void test_gc_minimal(EyExecutionContext *ctx __attribute__((unused))) {
    finalised = 0;
    EyGCRegion *gc = ey_runtime_gc_create();

    int *a = ey_runtime_gc_alloc(gc, sizeof(int), finaliser);
    ey_runtime_gc_remember_root_object(gc, a);
    *a = 1;

    ey_runtime_gc_collect(gc);

    if (finalised != 0) {
        ey_runtime_panic("test", "bad finaliser 1");
    }

    ey_runtime_gc_forget_root_object(gc, a);
    ey_runtime_gc_collect(gc);

    if (finalised != 1) {
        ey_runtime_panic("test", "bad finaliser 2");
    }

    if (ey_runtime_gc_get_stats(gc).bytes_allocated != 0) {
        ey_runtime_panic("test", "bad alloc 1");
    }

    ey_runtime_gc_free(gc);
}

static void test_gc_vector(EyExecutionContext *ctx) {
    EyGCRegion *gc = ey_runtime_gc(ctx);

    for (int pin_via_pointer = 0; pin_via_pointer < 2; pin_via_pointer += 1) {
        ey_runtime_gc_collect(gc);

        const int before = ey_runtime_allocated_bytes(ctx);
        const int initial_allocations = ey_runtime_gc_get_stats(gc).pages_allocated;

        EyString s1 = ey_runtime_string_create_literal(ctx, "abc"),
                 s2 = ey_runtime_string_create_literal(ctx, "def"),
                 s3 = ey_runtime_string_create_literal(ctx, "ghi");

        EyVector *v = ey_vector_create(ctx, sizeof(EyString));

        if (pin_via_pointer) {
            ey_runtime_gc_remember_root_pointer(gc, &v);
        } else {
            ey_runtime_gc_remember_root_object(gc, v);
        }

        ey_vector_append(ctx, v, &s1);
        ey_vector_append(ctx, v, &s2);
        ey_vector_append(ctx, v, &s3);

        const int after = ey_runtime_allocated_bytes(ctx);
        const int interim_allocations = ey_runtime_gc_get_stats(gc).pages_allocated;

        ey_runtime_gc_collect(gc);

        if (after != ey_runtime_allocated_bytes(ctx)) {
            ey_runtime_panic("test", "should not have deallocated vector");
        }
        if (interim_allocations != ey_runtime_gc_get_stats(gc).pages_allocated) {
            ey_runtime_panic("test", "should have remained at interim allocations");
        }

        if (pin_via_pointer) {
            ey_runtime_gc_forget_root_pointer(gc, &v);
        } else {
            ey_runtime_gc_forget_root_object(gc, v);
        }
        ey_runtime_gc_collect(gc);

        if (initial_allocations != ey_runtime_gc_get_stats(gc).pages_allocated) {
            ey_runtime_panic("test", "should have returned to initial allocations");
        }

        if (before != ey_runtime_allocated_bytes(ctx)) {
            ey_runtime_panic("test", "should have deallocated vector");
        }
    }
}

static void test_gc_stack(EyExecutionContext *ctx __attribute__((unused))) {
    finalised = 0;
    EyGCRegion *gc = ey_runtime_gc_create();

    int *a = ey_runtime_gc_alloc(gc, sizeof(int), finaliser);
    ey_runtime_gc_remember_root_pointer(gc, &a);
    *a = 1;

    int *b = ey_runtime_gc_alloc(gc, sizeof(int), finaliser);
    ey_runtime_gc_remember_root_pointer(gc, &b);

    if (ey_runtime_gc_get_stats(gc).bytes_allocated != sizeof(int) * 2) {
        ey_runtime_panic("test", "bad alloced 1");
    }

    ey_runtime_gc_collect(gc);

    if (finalised != 0) {
        ey_runtime_panic("test", "bad finaliser 1");
    }

    b = 0;
    ey_runtime_gc_forget_root_pointer(gc, &a);
    ey_runtime_gc_collect(gc);

    if (finalised != 1) {
        ey_runtime_panic("test", "bad finaliser 2");
    }

    if (ey_runtime_gc_get_stats(gc).bytes_allocated != 0) {
        ey_runtime_panic("test", "bad alloc 1");
    }

    ey_runtime_gc_free(gc);
}

typedef struct {
    int a;
    int *b;
} XY;

static void test_gc_recursive(EyExecutionContext *ctx __attribute__((unused))) {
    finalised = 0;
    EyGCRegion *gc = ey_runtime_gc_create();

    XY *xy = ey_runtime_gc_alloc(gc, sizeof(XY), finaliser);
    ey_runtime_gc_remember_root_object(gc, xy);
    xy->a = 1;
    xy->b = ey_runtime_gc_alloc(gc, sizeof(int), finaliser);
    *xy->b = 2;

    ey_runtime_gc_collect(gc);

    if (finalised != 0) {
        ey_runtime_panic("test_gc_recursive", "bad finaliser 1");
    }

    ey_runtime_gc_forget_root_object(gc, xy);
    ey_runtime_gc_collect(gc);

    if (finalised != 3) {
        ey_runtime_panic("test_gc_recursive", "bad finaliser 2");
    }

    if (ey_runtime_gc_get_stats(gc).bytes_allocated != 0) {
        ey_runtime_panic("test_gc_recursive", "bad alloc 1");
    }

    ey_runtime_gc_free(gc);
}

void ey_generated_main(EyExecutionContext *ctx) {
    printf("test_vector\n");
    test_vector(ctx);

    printf("test_gc_minimal\n");
    test_gc_minimal(ctx);

    printf("test_gc_vector\n");
    test_gc_vector(ctx);

    printf("test_gc_stack\n");
    test_gc_stack(ctx);

    printf("test_gc_recursive\n");
    test_gc_recursive(ctx);

    printf("test_basic_worker\n");
    test_basic_worker(ctx);

    printf("test_returning_worker\n");
    test_returning_worker(ctx);

    printf("test_pipeline\n");
    test_pipeline(ctx);

    printf("test_strings\n");
    test_strings(ctx);

    // gpu below
    ey_init_opencl(cl_src);
    if (ey_runtime_check_cl(ctx)) {
        printf("test_gpu_worker\n");
        test_gpu_worker(ctx);
        printf("test_gpu_worker_with_parameter\n");
        test_gpu_worker_with_parameter(ctx);
    } else {
        printf("CL runtime not found, skipping tests\n");
    }
}
