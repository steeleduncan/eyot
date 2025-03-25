/*
  CPU-side Eyot runtime definitions

  These are for code that can only run cpu-side
 */
#pragma once

#include "eyot-runtime-common.h"

/*
  This is filled with the cl runtime src when required
  Alternately if cl is not needed, it is nil
 */
extern const char *ey_runtime_cl_src;

/*
  - Pointer to input
  - Pointer to output
  - Pointer to context
 */
typedef void (*EyWorkerFunction)(EyExecutionContext *ey_execution_context, void *, void *, void *);

/*
  Entry point
  NB this is defined by the program itself
*/
void ey_generated_main(EyExecutionContext *ctx);

/*
  No return panic

  NB could do with the noreturn attribute for GCC, but that is not portable to MSVC
*/
__attribute__((noreturn)) void ey_runtime_panic(const char *unit, const char *msg);

/*
  Core log print call
  This includes a newline
 */
void ey_print(const char *msg, ...);

/*
  Do nothing
  Just a hack...
 */
static void ey_noop(EyExecutionContext *ctx __attribute__((unused))) {
}

/*
  A printing utility that adds line numbers (useful for broken shaders, etc)
 */
void ey_print_with_line_numbers(const char *src);

// allocate memory without zeroing
void *ey_runtime_manual_alloc(EyInteger size);

// reallocate memory without zeroing
void *ey_runtime_manual_realloc(void *ptr, EyInteger size);

void ey_runtime_manual_free(void *ptr);

/*
 *  Array type
 *
 *  The unit size is not required as the vector would always have type information
 *  However, it is convenient
 */

/*
  Allocate a new vector of 0 size
 */
EyVector *ey_vector_create(EyExecutionContext *ctx, int unit_size);

/*
  Update the vector's size
 */
void ey_vector_resize(EyExecutionContext *ctx, EyVector *vec, int new_length);

/*
  Return a pointer to an element in the runtime
 */
void *ey_vector_access(EyExecutionContext *ctx, EyVector *vec, int index);

/*
  Number of slots in a vector
 */
int ey_vector_length(EyExecutionContext *ctx, const EyVector *vec);

/*
  Append a new element to the vector
 */
void ey_vector_append(EyExecutionContext *ctx, EyVector *vec, const void *new_element);

/*
  Append an entire vector to the vector
 */
void ey_vector_append_vector(EyExecutionContext *ctx, EyVector *vec, EyVector *new_elements);

/*
  Get a pointer to the entire vector
 */
void *ey_vector_get_ptr(EyExecutionContext *ctx, EyVector *vec);

/*
  Erase a range of data from the vector
 */
void ey_vector_erase(EyExecutionContext *ctx, EyVector *vec, EyInteger start, EyInteger count);

/*
  Create a range vector

  This is intended to match `list(range(start, end, step))` in Python
 */
EyVector *ey_runtime_range(EyExecutionContext *ctx, EyInteger start, EyInteger end, EyInteger step);

/*
 * Workers
 */
typedef struct EyWorker {
    /*
    Send a vector of values to the worker input

    If it is not a void worker, the values would be written to the output
    */
    void (*send)(EyWorker *w, EyVector *values);

    /*
    Receive a single value from the worker

    This assumes that value points to a block of memory the same as the output size of the wrker
    */
    void (*receive)(EyWorker *w, void *value);

    /*
    This closes the worker, and pull all values before returning
    */
    EyVector *(*drain)(EyWorker *w);

    /*
      The output size of this worker
     */
    EyInteger output_size;
    void *ctx;
} EyWorker;

/*
 Create a new worker thread(for now)

 output_size can be 0 for a void worker, input size can not
 The context will be passed to the worker function
*/
EyWorker *ey_worker_create_cpu(EyWorkerFunction fn, int input_size, int output_size, void *ctx,
                               int ctx_size);

/*
  Create a pipeline

  For now this simply joins two workers in a background thread
 */
EyWorker *ey_worker_create_pipeline(EyWorker *lhs, EyWorker *rhs);

/*
 * Open CL
 */

/*
  Initialse open cl with the program source

  Currently we load and co mpile everything, then chop up the relevant kernels later
 */
void ey_init_opencl(const char *src);

/*
  Ensure OpenCL works
 */
EyBoolean ey_runtime_check_cl(EyExecutionContext *ey_execution_context);

/*
  Initialise OpenCl

  NB the parameter count is expected to have the same lif
 */
EyWorker *ey_worker_create_opencl(const char *kernel, int input_size, int output_size,
                                  void *closure_ptr, int closure_size);

/*
 * Closure
 */

/*
  Create a new closure
 */
EyClosure ey_closure_create(int fid, void **args);

/*
 * GC defs
 */

/*
  Opaque type for a single region of memory
 */
typedef struct EyGCRegion EyGCRegion;
typedef struct EyGCStats {
    int bytes_allocated;
    int pages_allocated;
} EyGCStats;

/*
  This would be called with the block of memory being deallocated

  NB this is called from within the GC's lock
     it is important not to call back into the GC from this
 */
typedef void (*Finaliser)(void *);

/*
  Create a new GC region
 */
EyGCRegion *ey_runtime_gc_create(void);

/*
  The global active GC
 */
EyGCRegion *ey_runtime_gc(EyExecutionContext *ctx);

/*
  Information on the gc
 */
EyGCStats ey_runtime_gc_get_stats(EyGCRegion *);

/*
  Completely tear down a region
 */
void ey_runtime_gc_free(EyGCRegion *);

/*
  Alloc a new block

  This will be zeroed
 */
void *ey_runtime_gc_alloc(EyGCRegion *region, int block_size, Finaliser fn);

/*
  Resize an existing block

  Any new memory area will be zeroed
 */
void *ey_runtime_gc_realloc(EyGCRegion *region, void *ptr, int new_size);

/*
  Increment root count on allocation

  This is the case you remember explicit root objects
  There is also ey_runtime_gc_remember_root_pointer
 */
void ey_runtime_gc_remember_root_object(EyGCRegion *region, void *ptr);

/*
  Decrement root count on allocation

  This is the case you remember explicit root objects
  There is also ey_runtime_gc_forget_root_pointer
 */
void ey_runtime_gc_forget_root_object(EyGCRegion *region, void *ptr);

/*
  Trigger a collection

  NB this assumes it is safe to do so
 */
void ey_runtime_gc_collect(EyGCRegion *region);

/*
  How much has been allocated

  NB this is exposed to eyot
 */
EyInteger ey_runtime_allocated_bytes(EyExecutionContext *ctx);

/*
  Save a stack pointer
 */
void ey_runtime_gc_remember_root_pointer(EyGCRegion *region, const void *ptr);

/*
  Save a stack pointer
 */
void ey_runtime_gc_forget_root_pointer(EyGCRegion *region, const void *ptr);

/*
  Trigger a collection

  NB this is exposed to eyot
 */
void ey_runtime_collect(EyExecutionContext *ctx);

/*
 * String functions
 */

/*
  Used when assigning a literal to a variable
  This may copy
 */
EyString ey_runtime_string_assign(EyExecutionContext *ctx, EyString s);

/*
  Create a copy of a string
 */
EyString ey_runtime_string_copy(EyExecutionContext *ctx, EyString s);

/*
  Add two strings together
 */
EyString ey_runtime_string_join(EyExecutionContext *ctx, EyString lhs, EyString rhs);

/*
  The number of USVs in a string
 */
EyInteger ey_runtime_string_character_length(EyExecutionContext *ctx, EyString s);

/*
  Check if two strings are equal
 */
EyBoolean ey_runtime_string_equality(EyExecutionContext *ctx, EyString lhs, EyString rhs);

/*
  Extract a unicode USV
 */
EyCharacter ey_runtime_string_get_character(EyExecutionContext *ctx, EyString s, int position);

/*
  Set a unicode USV
 */
void ey_runtime_string_set_character(EyExecutionContext *ctx, EyString s, int position, EyCharacter c);

/*
  Create a literal runtime string from a utf 8 string
 */
EyString ey_runtime_string_create_literal(EyExecutionContext *ctx, const char *literal);

/*
  Resize the string storage
 */
EyString ey_runtime_string_resize(EyExecutionContext *ctx, EyString s, EyInteger l);

// Get args passed on boot
EyVector *ey_runtime_get_args(EyExecutionContext *ctx);

/*
  String to utf8 string for syscalls
  This should be manually released after
 */
const char *ey_runtime_string_create_c_string(EyString eys);
