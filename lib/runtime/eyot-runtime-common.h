#define k_ey_max_arg_count EYOT_RUNTIME_MAX_ARGS

#define EYOT_RUNTIME_DEV_CHECKS 1

/*
  Core Eyot runtime definitions

  These are used on both the CPU and GPU code
 */
#pragma once

typedef int EyBoolean;
typedef int EyInteger;
#ifndef EYOT_RUNTIME_GPU
typedef double EyFloat64;
#endif
typedef float EyFloat32;

typedef struct EyPipe EyPipe;
typedef struct EyVector EyVector;
typedef struct EyWorker EyWorker;

#define k_true  1
#define k_false 0

/*
  String type
 */
typedef struct {
    /*
      The size of the data in this string
     */
    int length;

    /*
      Data pointer
     */
    void *ptr;

    /*
      When true this has static lifetime, and should never be deallocated
     */
    EyBoolean static_lifetime;
} EyStringS;
typedef EyStringS *EyString;

/*
  This is used by expanded for loops for comparing the iterator
 */
static EyBoolean ey_runtime_continue_iterating(EyInteger step, EyInteger lhs, EyInteger rhs) {
    if (step == 0) {
        // this is a bad case, never iterate further
        return k_false;
    } else if (step > 0) {
        return lhs < rhs;
    } else {
        return lhs > rhs;
    }
}

#ifdef EYOT_RUNTIME_GPU
// CL spec defines this as 32 bits
typedef int EyUint32;
#else
#include <stdint.h>
typedef uint32_t EyUint32;
#endif

typedef EyUint32 EyCharacter;

/*
  The closure type
 */
typedef void *EyClosure;

#define k_worker_buffer_size 1020

/*
  A shared data for worker output
 */
typedef struct {
    EyUint32 used;
    char buffer[k_worker_buffer_size];
} EyWorkerShared;

/*
  Execution context
 */
#ifdef EYOT_RUNTIME_GPU

typedef struct EyExecutionContext {
    EyStringS *strings;
    __global EyWorkerShared *shared;
} EyExecutionContext;

#else

typedef struct EyExecutionContext {
} EyExecutionContext;

#endif

/*
  Return the number of arguments for the specified fid

  This is generated as part of the runtime shims

  Pragmatically fid is restricted to the values of EyRuntimeFunctionList enum
  However that is defined during build
 */
int ey_generated_arg_count(int fid);

/*
  Call a function by its runtime id
 */
void ey_functioncaller(EyExecutionContext *ey_execution_context, int fid, void *result,
                       void **args);

/*
  This returns the size of any given argument

  This is generated as part of the runtime shims
 */
int ey_generated_closure_arg_size(int fid, int argument);

/*
  This returns the space given to the argument

  NB we need to align structs, etc

  For now this is
  TODO generate this, and pack more intelligently
 */
static int ey_generated_closure_arg_step_size(int fid, int argument) {
    int raw_size = ey_generated_closure_arg_size(fid, argument);

    while (raw_size % 8 > 0) {
        raw_size += 1;
    }

    return raw_size;
}

/*
  Extract the function id from the closure
 */
static int ey_closure_fid(EyClosure c) {
    return *(int *)c;
}

static int ey_closure_arg_exists_offset(int argument) {
    return 8  // TODO use sizeof(int); when alignment is sorted
           + 8 * argument;  // TODO use sizeof(EyBoolean); when alignment is sorted
}
/*
  Return a pointer to the EyBoolean for each argument describing source

  - true: the closure has the value
  - false: the value should be provided by the caller
 */
static EyBoolean ey_closure_arg_exists(EyClosure c, int argument) {
    unsigned char *ptr = c;
    ptr += ey_closure_arg_exists_offset(argument);
    return *(EyBoolean *)ptr;
}

// Setter for ey_closure_arg_exists
static void ey_closure_set_arg_exists(EyClosure c, int argument, EyBoolean value) {
    unsigned char *ptr = c;
    ptr += ey_closure_arg_exists_offset(argument);
    *(EyBoolean *)ptr = value;
}

/*
  Return an argument pointer from the closure
 */
static void *ey_closure_arg_pointer(EyClosure c, int argument) {
    const int fid = ey_closure_fid(c);
    unsigned char *ptr = c;
    ptr += 8;  // TODO use sizeof(int); when alignment is sorted
    ptr += 8 * ey_generated_arg_count(fid);  // TODO use sizeof(EyBoolean); when alignment is sorted
    for (int i = 0; i < argument; i += 1) {
        ptr += ey_generated_closure_arg_step_size(fid, i);
    }
    return (void *)ptr;
}

/*
  Return the overall size of the closure

  TODO generate a version of this, would be faster
 */
static int ey_generated_closure_size(int fid) {
    const int arg_count = ey_generated_arg_count(fid);
    int ret = 8;  // TODO sizeof(int); when alignment sorted
    for (int i = 0; i < arg_count; i += 1) {
        ret += ey_generated_closure_arg_step_size(fid, i);
        ret += 8;  // TODO sizeof(EyBoolean); when alignment fixed
    }
    return ret;
}

/*
  Simple memcpy implementation

  TODO this is not remotely efficient
 */
#ifdef EYOT_RUNTIME_GPU
static void ey_runtime_closure_copy(void *dest, __global const void *src) {
    const int fid = *(__global int *)src;
    const int len = ey_generated_closure_size(fid);

    unsigned char *d = (unsigned char *)dest;
    __global const unsigned char *s = (__global const unsigned char *)src;

    for (int i = 0; i < len; i += 1) {
        d[i] = s[i];
    }
}
#endif

/*
  Return the overall size of the closure
 */
static int ey_closure_size(EyClosure c) {
    return ey_generated_closure_size(ey_closure_fid(c));
}

/*
  Unpack arguments and call a closure
 */
static void ey_closure_call(EyExecutionContext *ey_execution_context, EyClosure c, void *result,
                            void **args) {
    const int fid = ey_closure_fid(c);

    void *resolved_args[k_ey_max_arg_count];
    const int arg_count = ey_generated_arg_count(fid);
    int passed_arg = 0;
    for (int i = 0; i < arg_count; i += 1) {
        void *aptr = ey_closure_arg_pointer(c, i);
        if (ey_closure_arg_exists(c, i)) {
            // this is provided by the closure
            resolved_args[i] = aptr;
        } else {
            resolved_args[i] = args[passed_arg];
            passed_arg += 1;
        }
    }

    ey_functioncaller(ey_execution_context, fid, result, resolved_args);
}

/*
  Convert a literal runtime string

  This really is a passthrough so we have the option to change later
 */
EyString ey_runtime_string_use_literal(EyExecutionContext *ctx, EyString literal);

/*
  Method that prints a single byte to the output

  All IO should drill down here
 */
void ey_print_byte(EyExecutionContext *ctx, char val);

/*
  Get a static string

  These need extracting from execution context to not upset CL
 */
EyString ey_runtime_string_get(EyExecutionContext *ctx, EyInteger string_index);

/*
  Some fns defined trivially GPU
 */
#ifdef EYOT_RUNTIME_GPU

void ey_print_byte(EyExecutionContext *ctx, char c) {
    __global EyWorkerShared *s = ctx->shared;

    if (s->used < k_worker_buffer_size) {
        s->buffer[s->used] = c;
        s->used += 1;
    }
}

EyString ey_runtime_string_use_literal(EyExecutionContext *ctx __attribute__((unused)),
                                       EyString literal) {
    return literal;
}

EyString ey_runtime_string_get(EyExecutionContext *ctx __attribute__((unused)),
                               EyInteger string_index) {
    return &ctx->strings[string_index];
}

#else

extern EyStringS *ey_string_pool_raw;

#endif

/*
  Print a block of data
 */
static void ey_print_block(EyExecutionContext *ctx, void *data, int length) {
    const unsigned char *ptr = data;
    for (int i = 0; i < length; i += 1) {
        ey_print_byte(ctx, ptr[i]);
    }
}

/*
  Core int printer
  The digits specifies the leading zeros
 */
static void ey_print_int_core(EyExecutionContext *ctx, EyInteger val, int leading_zeros) {
#define k_print_int_buf_size 40
    char buf[k_print_int_buf_size];

    if (val < 0) {
        ey_print_byte(ctx, '-');
        val = -val;
    }

    int i = 0;
    while (val > 0) {
        const int rem = val % 10;
        buf[i] = rem + '0';
        val /= 10;
        i += 1;
    }

    if (i == 0) {
        if (leading_zeros == 0) {
            ey_print_byte(ctx, '0');
        } else {
            for (int j = 0; j < leading_zeros; j += 1) {
                ey_print_byte(ctx, '0');
            }
        }
    } else {
        for (int j = i; j < leading_zeros; j += 1) {
            ey_print_byte(ctx, '0');
        }

        while (i > 0) {
            i -= 1;
            ey_print_byte(ctx, buf[i]);
        }
    }
#undef k_print_int_buf_size
}

static void ey_print_int(EyExecutionContext *ctx, EyInteger val) {
    ey_print_int_core(ctx, val, 0);
}

#ifndef EYOT_RUNTIME_GPU
static void ey_print_float64(EyExecutionContext *ctx, EyFloat64 val) {
    if (val < 0) {
        ey_print_byte(ctx, '-');
        val *= -1.0;
    }

    const int integral = (int)val;
    EyFloat64 fractional = val - (EyFloat64)integral;

    ey_print_int_core(ctx, integral, 0);
    ey_print_byte(ctx, '.');
    ey_print_int_core(ctx, (int)(fractional * 1000000.0), 6);
}
#endif

static void ey_print_float32(EyExecutionContext *ctx, EyFloat32 val) {
    if (val < 0) {
        ey_print_byte(ctx, '-');
        val *= -1.0;
    }

    const int integral = (int)val;
    EyFloat32 fractional = val - (EyFloat32)integral;

    ey_print_int_core(ctx, integral, 0);
    ey_print_byte(ctx, '.');
    ey_print_int_core(ctx, (int)(fractional * 1000000.0), 6);
}

static void ey_print_character(EyExecutionContext *ctx, EyCharacter ccode) {
    EyUint32 code = ccode;
    // snippet from https://gist.github.com/tylerneylon/9773800
    char val[4];
    EyUint32 lead_byte_max = 0x7F;
    EyUint32 val_index = 0;

    while (code > lead_byte_max) {
        val[val_index++] = (code & 0x3F) | 0x80;
        code >>= 6;
        lead_byte_max >>= (val_index == 1 ? 2 : 1);
    }

    val[val_index++] = (code & lead_byte_max) | (~lead_byte_max << 1);
    val[val_index] = 0;

    int i = 0;
    char rval[4] = {0, 0, 0, 0};
    while (val_index--) {
        rval[i] = val[val_index];
        i += 1;
    }

    ey_print_block(ctx, rval, i);
}

static void ey_print_boolean(EyExecutionContext *ctx, EyBoolean val) {
    /*
      The longhand is to avoid CL errors moving const strings to private pointer spaces
     */
    if (val) {
        ey_print_byte(ctx, 't');
        ey_print_byte(ctx, 'r');
        ey_print_byte(ctx, 'u');
        ey_print_byte(ctx, 'e');
    } else {
        ey_print_byte(ctx, 'f');
        ey_print_byte(ctx, 'a');
        ey_print_byte(ctx, 'l');
        ey_print_byte(ctx, 's');
        ey_print_byte(ctx, 'e');
    }
}

static void ey_print_string(EyExecutionContext *ctx, EyString val) {
    if (!val) {
        return;
    }

    EyCharacter *s = val->ptr;
    for (int i = 0; i < val->length / 4; i += 1) {
        ey_print_character(ctx, s[i]);
    }
}

static void ey_print_nl(EyExecutionContext *ctx) {
    ey_print_byte(ctx, 10);
}
