/*
  OpenCL Eyot runtime definitions

  This creates an EyWorker instance that uses CL to run on any attached GPUs
 */

#include "eyot-runtime-cpu.h"

#if defined(EYOT_OPENCL_INCLUDED)

#include <pthread.h>
#include <string.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>

/*
  This is pretty ancient now
  It is the mandatory baseline for OpenCL 3.0
 */
#define CL_TARGET_OPENCL_VERSION 120

#ifdef __APPLE__
#include <OpenCL/cl.h>
#else
#include <CL/cl.h>
#endif

typedef struct {
    cl_device_id device_id;
    cl_context context;

    /*
      It is a single program covering all our code, the kernels are fished out of here on worker
      start At the very list it makes it quicker to kick of workers
    */
    cl_program program;

    EyBoolean verbose;
} ClDriver;

static void print_with_line_numbers(const char *src) {
    int line_number = 1;
    const int len = strlen(src);

    int show_number = 1;

    for (int i = 0; i < len; i += 1) {
        if (show_number) {
            ey_print("%i: ", line_number);
            show_number = 0;
            line_number += 1;
        }

        const char c = src[i];
        ey_print("%c", c);

        if (c == 10) {
            show_number = 1;
        }
    }
}

static void cldriver_finalise(void *obj) {
    ClDriver *driver = obj;
    if (driver->program) {
        clReleaseProgram(driver->program);
    }
    if (driver->context) {
        clReleaseContext(driver->context);
    }
}

static ClDriver *cldriver_create(const char *src) {
    const char *disable_sentinel = getenv("EyotDisableCl");
    if (disable_sentinel && strcmp(disable_sentinel, "y") == 0) {
        return 0;
    }

    EyBoolean verbose = k_false;
    const char *verbose_sentinel = getenv("EyotVerbose");
    if (verbose_sentinel && strcmp(verbose_sentinel, "y") == 0) {
        verbose = k_true;
    }

    if (verbose) {
        ey_print(src);
    }

    cl_uint nplatforms;
    cl_int err = clGetPlatformIDs(0, NULL, &nplatforms);
    if (err != CL_SUCCESS) {
        // this (an) expected failure case when cl is installed, but there are no platforms
        // stay silent so we don't polllute the log
        if (verbose) {
            ey_print("cldriver_create: clGetPlatformIDs (1) failed with %i\n", err);
        }
        return 0;
    }

    if (nplatforms == 0) {
        ey_print("cldriver_create: no cl platforms found\n");
        return 0;
    }

    cl_platform_id *platforms = (cl_platform_id *)malloc(sizeof(cl_platform_id) * nplatforms);
    if (!platforms) {
        ey_runtime_panic("cldriver_create", "failed to allocate platform list\n");
    }

    err = clGetPlatformIDs(nplatforms, platforms, NULL);
    if (err != CL_SUCCESS) {
        ey_print("cldriver_create: clGetPlatformIDs (2) failed with %i\n", err);
        return 0;
    }

    ClDriver *driver = ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(ClDriver), cldriver_finalise);
    if (!driver) {
        ey_runtime_panic("cldriver_create", "failed to allocate driver structure\n");
    }
    driver->verbose = verbose;

    if (driver->verbose) {
        ey_print("OpenCL driver initialising. %d platforms found (will choose 0)\n", nplatforms);
        for (cl_uint i = 0; i < nplatforms; i++) {
            char name[128], vendor[128], version[128];

            err |= clGetPlatformInfo(platforms[i], CL_PLATFORM_VENDOR, 128, vendor, NULL);
            err |= clGetPlatformInfo(platforms[i], CL_PLATFORM_NAME, 128, name, NULL);
            err |= clGetPlatformInfo(platforms[i], CL_PLATFORM_VERSION, 128, version, NULL);

            ey_print("  %d: %s %s %s\n", i, vendor, name, version);
        }
    }

    err = clGetDeviceIDs(platforms[0], CL_DEVICE_TYPE_GPU, 1, &driver->device_id, NULL);
    if (err != CL_SUCCESS) {
        if (driver->verbose) {
            // this (an) expected failure case when cl is installed, but there are no viable
            // devices, so stay silent by default
            ey_print("cldriver_create: clGetDeviceIDs failed with %i\n", err);
        }
        return 0;
    }

    driver->context = clCreateContext(0, 1, &driver->device_id, NULL, NULL, &err);
    if (!driver->context) {
        ey_print("cldriver_create: clCreateContext failed with %i\n", err);
        return 0;
    }

    // compile the single source
    driver->program = clCreateProgramWithSource(driver->context, 1, &src, 0, &err);
    if (!driver->program) {
        ey_runtime_panic("cldriver_create", "failed to create program");
    }

    if (driver->verbose) {
        ey_print(src);
    }
    err = clBuildProgram(driver->program, 0, NULL, NULL, NULL, NULL);
    if (err != CL_SUCCESS) {
        size_t len;

        print_with_line_numbers(src);
        ey_print("cldriver_create: Failed to build program executable!\n");

        err = clGetProgramBuildInfo(driver->program, driver->device_id, CL_PROGRAM_BUILD_LOG, 0, 0,
                                    &len);
        if (err != CL_SUCCESS) {
            ey_runtime_panic("cldriver_create",
                             "failed to compile program and got error when checking build length");
        }
        void *build_log = ey_runtime_manual_alloc(len + 1);
        err = clGetProgramBuildInfo(driver->program, driver->device_id, CL_PROGRAM_BUILD_LOG, len,
                                    build_log, 0);
        if (err != CL_SUCCESS) {
            ey_runtime_panic("cldriver_create",
                             "failed to compile program and got error when reading build log");
        }
        ey_print("%s\n", build_log);
        ey_runtime_panic("cldriver_create", "failed to compile program");
        ey_runtime_manual_free(build_log);
    }

    return driver;
}

static ClDriver *_singleton_driver = 0;
void ey_init_opencl(const char *src) {
    if (!src || strlen(src) == 0) {
        _singleton_driver = 0;
    } else {
        _singleton_driver = cldriver_create(src);
    }
}

typedef struct {
    cl_mem input, output;
    EyVector *output_vector;
    cl_event evt_done;

    /*
      Number of items in this batch
     */
    size_t count;

    /*
      The read index

      Before read starts on this, it is negative
      After that, it is the first index in the batch that is new
     */
    int read_index;
} WorkBatch;

typedef struct {
    pthread_mutex_t mutex;

    cl_command_queue command_queue;
    cl_kernel kernel;

    // all batches, and the size of the allocation
    WorkBatch *batches;
    int batches_allocated;

    // batches currently in use
    int batches_used;

    // data unit sizes for in and out
    int input_size, output_size;

    // core driver
    ClDriver *driver;

    // a (possibly null) pointer to a closure
    void *closure;
    cl_mem closure_buffer;

    // the size of the closure object
    int closure_size;

    // local workgroup size
    size_t local_workgroup_size;

    // An event for when the worker is ready
    cl_event ready_event;

    /*
      Number of copied parameters
     */
    int parameter_count;

    /*
      Shared buffers with the workers
     */
    cl_mem shared_buffers_gpu;
    EyWorkerShared *shared_buffers_host;

    /*
      The last known used pointer for the buffer
     */
    int *buffer_used;

    // number of awaited results
    int activity_count;
} EyClWorker;

/*
  Create a new batch, and return a pointer to it
 */
static WorkBatch *clworker_new_batch(EyClWorker *clw) {
    clw->batches_used += 1;
    while (clw->batches_allocated < clw->batches_used) {
        clw->batches_allocated = clw->batches_used;
    }
    clw->batches = ey_runtime_gc_realloc(ey_runtime_gc(0), clw->batches,
                                         sizeof(WorkBatch) * clw->batches_allocated);
    return &clw->batches[clw->batches_used - 1];
}

static void clworker_pop_batch(EyClWorker *clw) {
    if (clw->batches_used == 0) {
        ey_runtime_panic("clworker_pop_batch", "no batch found");
    }

    clReleaseMemObject(clw->batches[0].input);
    clReleaseMemObject(clw->batches[0].output);
    if (clw->closure) {
        clReleaseMemObject(clw->shared_buffers_gpu);
    }
    if (clw->closure) {
        clReleaseMemObject(clw->closure_buffer);
    }

    for (int i = 1; i < clw->batches_used; i += 1) {
        clw->batches[i - 1] = clw->batches[i];
    }
    clw->batches_used -= 1;
}

static int round_up(int value, int divisor) {
    const int div = value / divisor, remainder = value % divisor;

    if (remainder == 0) {
        return div * divisor;
    } else {
        return (div + 1) * divisor;
    }
}

static int ey_cl_worker_shared_buffer_size(EyClWorker *w) {
    return sizeof(EyWorkerShared) * w->local_workgroup_size;
}

static void ey_cl_send(EyWorker *wrkr, EyVector *values) {
    EyClWorker *w = wrkr->ctx;
    pthread_mutex_lock(&w->mutex);

    cl_int err;
    cl_event computation_finished_event;

    WorkBatch *batch = clworker_new_batch(w);
    *batch = (WorkBatch){
        .read_index = -1,
        .count = ey_vector_length(0, values),
        .output_vector = ey_vector_create(0, w->output_size),
    };
    ey_vector_resize(0, batch->output_vector, batch->count);

    w->activity_count += batch->count;

    // TODO array these, we only support a single read/write pair RN
    batch->input = clCreateBuffer(w->driver->context, CL_MEM_READ_ONLY,
                                  w->input_size * batch->count, NULL, NULL);
    batch->output = clCreateBuffer(w->driver->context, CL_MEM_WRITE_ONLY,
                                   w->output_size * batch->count, NULL, NULL);
    if (!batch->input || !batch->output) {
        ey_runtime_panic("ey_cl_send", "failed to allocate io memory");
    }

    cl_event input_written_event;
    err = clEnqueueWriteBuffer(w->command_queue, batch->input, CL_TRUE, 0,
                               w->input_size * batch->count, ey_vector_get_ptr(0, values), 1,
                               &w->ready_event, &input_written_event);
    if (err != CL_SUCCESS) {
        ey_runtime_panic("ey_cl_send", "failed to write input memory");
    }

    // Set the arguments to our compute kernel
    // these are "fixed" parameters
    err = clSetKernelArg(w->kernel, 0, sizeof(cl_mem), &batch->input);
    if (err != CL_SUCCESS) {
        ey_runtime_panic("ey_cl_send", "failed to set input pointer");
    }

    err = clSetKernelArg(w->kernel, 1, sizeof(cl_mem), &batch->output);
    if (err != CL_SUCCESS) {
        ey_runtime_panic("ey_cl_send", "failed to set output pointer");
    }

    const cl_uint uic = (cl_uint)batch->count;
    err = clSetKernelArg(w->kernel, 2, sizeof(unsigned int), &uic);
    if (err != CL_SUCCESS) {
        ey_runtime_panic("ey_cl_send", "failed to set count value");
    }

    err = clSetKernelArg(w->kernel, 3, sizeof(cl_mem), &w->shared_buffers_gpu);
    if (err != CL_SUCCESS) {
        ey_runtime_panic("ey_cl_send", "failed to set shared buffers pointer");
    }

    if (w->closure) {
        err = clSetKernelArg(w->kernel, 4, sizeof(cl_mem), &w->closure_buffer);
        if (err != CL_SUCCESS) {
            if (err == CL_INVALID_MEM_OBJECT) {
                ey_runtime_panic("ey_cl_send", "invalid memory object");
            }

            ey_runtime_panic("ey_cl_send", "failed to set closure pointer");
        }
    }

    /*
      NB
      - global workgroup size must be a multiple of local workgroup size
      - the kernel follows the count parameter, not
    */
    size_t global_workgroup_size = round_up(batch->count, w->local_workgroup_size);
    err = clEnqueueNDRangeKernel(w->command_queue, w->kernel, 1, NULL, &global_workgroup_size,
                                 &w->local_workgroup_size, 1, &input_written_event,
                                 &computation_finished_event);
    if (err != CL_SUCCESS) {
        if (err == CL_INVALID_WORK_GROUP_SIZE) {
            ey_runtime_panic("ey_cl_send", "invalid work group size");
        } else if (err == CL_INVALID_KERNEL_ARGS) {
            ey_runtime_panic("ey_cl_send", "invalid kernel args");
        } else {
            ey_print("error code %i\n", err);
        }
        ey_runtime_panic("ey_cl_send", "failed to dispatch kernel");
    }

    cl_event output_read_event;

    // read the output
    err = clEnqueueReadBuffer(w->command_queue, batch->output, CL_TRUE, 0,
                              w->output_size * batch->count,
                              ey_vector_get_ptr(0, batch->output_vector), 1,
                              &computation_finished_event, &output_read_event);
    if (err != CL_SUCCESS) {
        ey_runtime_panic("ey_cl_send", "failed to read output buffer");
    }

    const int shared_buffer_size = ey_cl_worker_shared_buffer_size(w);

    // read the log
    err =
        clEnqueueReadBuffer(w->command_queue, w->shared_buffers_gpu, CL_TRUE, 0, shared_buffer_size,
                            w->shared_buffers_host, 1, &output_read_event, &batch->evt_done);
    if (err != CL_SUCCESS) {
        printf("%i\n", err);
        ey_runtime_panic("ey_cl_send", "failed to read log buffer");
    }

    pthread_mutex_unlock(&w->mutex);
}

/*
  Push the logs to stdout

  NB this assumes it has already been locked
 */
static void _ey_cl_pump_logs(EyExecutionContext *ctx, EyClWorker *w) {
    for (size_t i = 0; i < w->local_workgroup_size; i += 1) {
        EyWorkerShared *s = &w->shared_buffers_host[i];
        const int used = s->used;
        int last_nl = -1;
        for (int j = w->buffer_used[i]; j < used; j += 1) {
            if (s->buffer[j] == 10) {
                last_nl = j + 1;
            }
        }

        if (last_nl >= 0) {
            EyBoolean show_source = k_true;
            for (int j = w->buffer_used[i]; j < last_nl; j += 1) {
                if (show_source) {
                    ey_print_block(ctx, "(gpu ", 5);
                    ey_print_int(ctx, i);
                    ey_print_character(ctx, ')');
                    ey_print_character(ctx, ' ');
                    show_source = k_false;
                }
                ey_print_byte(ctx, s->buffer[j]);
                if (s->buffer[j] == 10) {
                    show_source = k_true;
                }
            }
            w->buffer_used[i] = last_nl;
        }
    }
}

/*
  Reset the log buffer to make space for more
 */
static void _ey_clear_logs(EyClWorker *w, EyBoolean wait_on_event) {
    const int shared_buffer_size = ey_cl_worker_shared_buffer_size(w);

    int wait_count = 0;
    const cl_event *wait_list = 0;
    if (wait_on_event) {
        /*
          NB the count and nullness of the pointer must match according to spec
          macOS driver cares, the linux one seemed not to
        */
        wait_count = 1;
        wait_list = &w->ready_event;
    }

    cl_event new_ready_event;
    memset(w->shared_buffers_host, 0, shared_buffer_size);
    const cl_int err = clEnqueueWriteBuffer(w->command_queue, w->shared_buffers_gpu, CL_TRUE, 0,
                                            shared_buffer_size, w->shared_buffers_host, wait_count,
                                            wait_list, &new_ready_event);

    // NB don't want in and out to be pointers to same event
    w->ready_event = new_ready_event;
    if (err != CL_SUCCESS) {
        if (err == CL_INVALID_EVENT_WAIT_LIST) {
            ey_print("code CL_INVALID_EVENT_WAIT_LIST\n");
        } else {
            ey_print("code = %i\n", err);
        }
        ey_runtime_panic("_ey_clear_logs", "failed to write shared buffers");
    }
    for (size_t i = 0; i < w->local_workgroup_size; i += 1) {
        w->buffer_used[i] = 0;
    }
}

/*
  Called when activity_count goes down and we have an opportunity to clear log buffers
 */
static void _ey_activity_count_reduced(EyClWorker *w) {
    if (w->activity_count > 0) {
        return;
    }

    EyBoolean log_used = k_false;
    for (size_t i = 0; i < w->local_workgroup_size; i += 1) {
        if (w->buffer_used[i]) {
            log_used = k_true;
        }
    }

    if (log_used) {
        _ey_clear_logs(w, k_true);
    }
}

static void ey_cl_receive(EyWorker *wrkr, void *value) {
    EyClWorker *w = wrkr->ctx;
    pthread_mutex_lock(&w->mutex);

    WorkBatch *batch = &w->batches[0];
    if (batch->read_index < 0) {
        clWaitForEvents(1, &batch->evt_done);
        _ey_cl_pump_logs(0, w);
        batch->read_index = 0;

        w->activity_count -= batch->count;
        _ey_activity_count_reduced(w);
    }

    memcpy(value, ey_vector_access(0, batch->output_vector, batch->read_index), w->output_size);
    batch->read_index += 1;

    if (batch->read_index == (int)batch->count) {
        clworker_pop_batch(w);
    }

    pthread_mutex_unlock(&w->mutex);
}

static EyVector *ey_cl_drain(EyWorker *wrkr) {
    EyClWorker *w = wrkr->ctx;
    pthread_mutex_lock(&w->mutex);

    WorkBatch *last_batch = &w->batches[w->batches_used - 1];
    if (last_batch->read_index < 0) {
        clWaitForEvents(1, &last_batch->evt_done);
        _ey_cl_pump_logs(0, w);
    }

    EyVector *vec = ey_vector_create(0, w->output_size);

    for (int i = 0; i < w->batches_used; i += 1) {
        WorkBatch *batch = &w->batches[i];

        if (batch->read_index < 0) {
            ey_vector_append_vector(0, vec, batch->output_vector);
        } else {
            for (; batch->read_index < (int)batch->count; batch->read_index += 1) {
                ey_vector_append(0, vec,
                                 ey_vector_access(0, batch->output_vector, batch->read_index));
            }
        }
    }

    w->batches_used = 0;
    if (w->closure) {
        clReleaseMemObject(w->closure_buffer);
    }

    w->activity_count -= ey_vector_length(0, vec);
    _ey_activity_count_reduced(w);
    pthread_mutex_unlock(&w->mutex);

    ey_runtime_gc_forget_root_object(ey_runtime_gc(0), w);

    return vec;
}

static void ey_cl_worker_finalise(void *obj) {
    EyClWorker *wrkr = obj;
    pthread_mutex_destroy(&wrkr->mutex);
    if (wrkr->command_queue) {
        clReleaseCommandQueue(wrkr->command_queue);
    }
    if (wrkr->kernel) {
        clReleaseKernel(wrkr->kernel);
    }
}

EyWorker *ey_worker_create_opencl(const char *kernel_name, int input_size, int output_size,
                                  void *closure_ptr, int closure_size) {
    if (!_singleton_driver) {
        ey_runtime_panic("ey_worker_create_opencl", "CL has not been initialised");
        return 0;
    }

    cl_int err;
    EyClWorker *wrkr =
        ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(EyClWorker), ey_cl_worker_finalise);
    if (!wrkr) {
        ey_runtime_panic("ey_worker_create_opencl", "failed to allocate cl worker structure");
    }

    static const int workgroup_size = 64, initial_batch_count = 10;

    *wrkr = (EyClWorker){
        .activity_count = 0,
        .batches =
            ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(WorkBatch) * initial_batch_count, 0),
        .batches_used = 0,
        .batches_allocated = initial_batch_count,
        .input_size = input_size,
        .output_size = output_size,
        .driver = _singleton_driver,
        .closure = closure_ptr,
        .closure_size = closure_size,
        .local_workgroup_size = workgroup_size,
    };
    pthread_mutex_init(&wrkr->mutex, 0);

    if (!wrkr->driver) {
        // from this point on there are no elegant failure options, so just panic
        ey_runtime_panic("ey_worker_create_opencl", "no cl driver found");
    }

    wrkr->command_queue =
        clCreateCommandQueue(wrkr->driver->context, wrkr->driver->device_id, 0, &err);
    if (!wrkr->command_queue) {
        ey_runtime_panic("ey_worker_create_opencl", "failed to create command queue");
    }

    wrkr->kernel = clCreateKernel(wrkr->driver->program, kernel_name, &err);
    if (!wrkr->kernel || err != CL_SUCCESS) {
        ey_runtime_panic("ey_worker_create_opencl", "Failed to create compute kernel!");
    }

    // we dont have a custom deallocator for the worker as the real complexity is in the ctx
    EyWorker *w = ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(EyWorker), 0);
    if (!w) {
        ey_runtime_panic("ey_worker_create_opencl", "failed to allocate worker structure");
    }
    *w = (EyWorker){
        .send = ey_cl_send,
        .receive = ey_cl_receive,
        .drain = ey_cl_drain,
        .output_size = output_size,
        .ctx = wrkr,
    };
    ey_runtime_gc_remember_root_object(ey_runtime_gc(0), w);

    const int shared_buffer_size = ey_cl_worker_shared_buffer_size(wrkr);

    wrkr->buffer_used =
        ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(int) * wrkr->local_workgroup_size, 0);
    wrkr->shared_buffers_host = ey_runtime_gc_alloc(ey_runtime_gc(0), shared_buffer_size, 0);
    wrkr->shared_buffers_gpu =
        clCreateBuffer(wrkr->driver->context, CL_MEM_READ_WRITE, shared_buffer_size, NULL, NULL);

    _ey_clear_logs(wrkr, k_false);

    if (closure_ptr) {
        wrkr->closure_buffer =
            clCreateBuffer(wrkr->driver->context, CL_MEM_WRITE_ONLY, closure_size, NULL, NULL);

        cl_event wait_event = wrkr->ready_event;

        // what does it imply about the lifetime of the closure?
        err = clEnqueueWriteBuffer(wrkr->command_queue, wrkr->closure_buffer, CL_TRUE, 0,
                                   closure_size, closure_ptr, 1, &wait_event, &wrkr->ready_event);
        if (err != CL_SUCCESS) {
            ey_runtime_panic("ey_cl_send", "failed to write input memory");
        }
    }

    return w;
}

EyBoolean ey_runtime_check_cl(EyExecutionContext *ey_execution_context __attribute__((unused))) {
    return _singleton_driver != 0;
}

#else  // EYOT_OPENCL_INCLUDED

EyWorker *ey_worker_create_opencl(const char *kernel __attribute__((unused)),
                                  int input_size __attribute__((unused)),
                                  int output_size __attribute__((unused)),
                                  void *closure_ptr __attribute__((unused)),
                                  int closure_size __attribute__((unused))) {
    return 0;
}

EyBoolean ey_runtime_check_cl(EyExecutionContext *ey_execution_context __attribute__((unused))) {
    // not really true, actually means it is irrelevant
    return k_false;
}

#endif  // EYOT_OPENCL_INCLUDED
