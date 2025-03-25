/*
  Eyot threadsafe pipe implementation
 */
#include "eyot-runtime-pipe.h"
#include "eyot-runtime-cpu.h"
#include <pthread.h>
#include <string.h>

#ifdef __APPLE__
#include <dispatch/dispatch.h>
#else
#include <semaphore.h>
#endif

/*
  The initially allocated size of a pipe
 */
static const int k_pipe_allocated_size = 3;

/*
  Pipes

  A locking data structure to pass data in a threadsafe manner
 */
typedef struct EyPipe {
    EyBoolean closed;

    /*
      The mutex to protect the data
     */
    pthread_mutex_t mutex;

    /*
      The sem that this pipe locks on
     */
#ifdef __APPLE__
    dispatch_semaphore_t semaphore;
#else
    sem_t semaphore;
#endif

    /*
      Size of a single value in the array
     */
    int value_size;

    /*
      Allocated size of the array
     */
    int allocated_size;

    /*
      Used size of the array
     */
    int used_size;

    /*
      The values in the pipe
     */
    void *values;
} EyPipe;

EyPipe *ey_pipe_create(int value_size) {
    EyPipe *p = ey_runtime_gc_alloc(ey_runtime_gc(0), sizeof(EyPipe), 0);
    if (p == 0) {
        ey_runtime_panic("ey_pipe_create", "unable to allocate");
    }
    *p = (EyPipe){
        .value_size = value_size,
        .allocated_size = k_pipe_allocated_size,
        .values = ey_runtime_gc_alloc(ey_runtime_gc(0), value_size * k_pipe_allocated_size, 0),
        .used_size = 0,
        .closed = k_false,
    };
    if (p->values == 0) {
        ey_runtime_panic("ey_pipe_create", "unable to allocate values");
    }
#ifdef __APPLE__
    p->semaphore = dispatch_semaphore_create(0);
#else
    if (sem_init(&p->semaphore, 0, 0) < 0) {
        ey_runtime_panic("ey_pipe_create", "sem_init failed");
    }
#endif

    return p;
}

void *ey_pipe_at(EyPipe *p, int i) {
    return (void *)((uint8_t *)p->values + p->value_size * i);
}

void ey_pipe_send(EyPipe *p, const void *value) {
    pthread_mutex_lock(&p->mutex);
    if (p->closed) {
        ey_runtime_panic("ey_pipe_send", "sending on a closed pipe");
    }

    if (p->allocated_size == p->used_size) {
        p->allocated_size += 1;
        p->values =
            ey_runtime_gc_realloc(ey_runtime_gc(0), p->values, p->allocated_size * p->value_size);
        if (!p->values) {
            ey_runtime_panic("ey_pipe_send", "reallocation of pipe failed");
        }
    }

    void *ptr = ey_pipe_at(p, p->used_size);
    p->used_size += 1;
    memcpy(ptr, value, p->value_size);
    pthread_mutex_unlock(&p->mutex);

#ifdef __APPLE__
    dispatch_semaphore_signal(p->semaphore);
#else
    sem_post(&p->semaphore);
#endif
}

EyBoolean ey_pipe_receive(EyPipe *p, void *value) {
    EyBoolean rv;
#ifdef __APPLE__
    dispatch_semaphore_wait(p->semaphore, DISPATCH_TIME_FOREVER);
#else
    sem_wait(&p->semaphore);
#endif

    pthread_mutex_lock(&p->mutex);
    if (p->closed && p->used_size == 0) {
        rv = k_false;
    } else {
        memcpy(value, ey_pipe_at(p, 0), p->value_size);
        p->used_size -= 1;
        if (p->used_size > 0) {
            memmove(ey_pipe_at(p, 0), ey_pipe_at(p, 1), p->value_size * p->used_size);
        }
        rv = k_true;
    }

    pthread_mutex_unlock(&p->mutex);
    return rv;
}

EyVector *ey_pipe_receive_multiple(EyPipe *p, int count) {
    EyVector *v = ey_vector_create(0, p->value_size);

    for (int i = 0; i < count; i += 1) {
        ey_vector_append(0, v, 0);
        void *ptr = ey_vector_access(0, v, i);

        if (!ey_pipe_receive(p, ptr)) {
            return 0;
        }
    }

    return v;
}

void ey_pipe_close(EyPipe *p) {
    pthread_mutex_lock(&p->mutex);
    p->closed = k_true;
    pthread_mutex_unlock(&p->mutex);

#ifdef __APPLE__
    dispatch_semaphore_signal(p->semaphore);
#else
    sem_post(&p->semaphore);
#endif
}
