/*
  Eyot Garbage collector
 */

#include "eyot-runtime-cpu.h"

#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <pthread.h>

// should disable this one day
const char *__asan_default_options(void) {
    return "detect_leaks=0";
}

// clearly this is platform dependent
static const int k_pointer_alignment = 8;

typedef struct PageHeader {
    // linked list
    struct PageHeader *next, *prev;

    // Finaliser
    Finaliser finaliser;

    // Size of this allocation
    int size;

    // > 0 root count implies this is a root that should be preserved
    int root_count;

    // all marked
    EyBoolean marked;
} PageHeader;

/*
  Convert page header to a page
 */
static void *gc_ptr_from_page(PageHeader *page) {
    return (void *)page + sizeof(PageHeader);
}

/*
  Convert page to a page header
 */
static PageHeader *gc_page_from_ptr(void *ptr) {
    return ptr - sizeof(PageHeader);
}

typedef struct {
    EyBoolean in_use;
    const void *pointer_to_pointer;
} EyStackPointer;

typedef struct EyGCRegion {
    // The lead page pointer in the linked list of pages
    PageHeader *root_page;

    EyGCStats stats;
    EyStackPointer *pointers;
    int pointers_allocated;
    pthread_mutex_t mutex;
} EyGCRegion;

EyGCStats ey_runtime_gc_get_stats(EyGCRegion *region) {
    return region->stats;
}

EyGCRegion *ey_runtime_gc_create(void) {
    EyGCRegion *region = ey_runtime_manual_alloc(sizeof(EyGCRegion));
    *region = (EyGCRegion){
        .root_page = 0,
        .stats =
            {
                .pages_allocated = 0,
                .bytes_allocated = 0,
            },
        .pointers_allocated = 10,
    };
    pthread_mutex_init(&region->mutex, 0);

    region->pointers = ey_runtime_manual_alloc(sizeof(EyStackPointer) * region->pointers_allocated);
    for (int i = 0; i < region->pointers_allocated; i += 1) {
        region->pointers[i] = (EyStackPointer){
            .in_use = k_false,
        };
    }

    if (sizeof(PageHeader) % k_pointer_alignment != 0) {
        printf("gc_int: bad page header size\n");
        exit(1);
    }

    return region;
}

/*
  Check if a pointer belongs to us
  NB we can't be sure that the page is real, so no accesses on it
 */
static EyBoolean gc_owns_ptr(EyGCRegion *region, void *ptr) {
    if ((uint64_t)ptr < sizeof(PageHeader)) {
        // checking like this avoids a runtime rollover error
        return k_false;
    }

    PageHeader *theoretical_page = gc_page_from_ptr(ptr);

    PageHeader *ph = region->root_page;
    while (ph) {
        if (theoretical_page == ph) {
            return k_true;
        }
        ph = ph->next;
    }

    return k_false;
}

static void gc_log(const EyGCRegion *region) {
    ey_print("start gc_log %p\n", region);
    const PageHeader *ph = region->root_page;
    while (ph) {
        ey_print(" - %p (%i) follows %p (marked = %i)\n", ph, ph->size, ph->prev, (int)ph->marked);
        ph = ph->next;
    }
}

// called locked
static void gc_check(EyGCRegion *region, const char *label) {
    static EyBoolean gc_check_enabled_set = k_false;
    static EyBoolean gc_check_enabled = k_false;

    if (!gc_check_enabled_set) {
        gc_check_enabled_set = k_true;

        const char *flag = getenv("EyotDebug");
        if (flag && strcmp(flag, "y") == 0) {
            gc_check_enabled = k_true;
        }
    }

    if (gc_check_enabled) {
        const PageHeader *prev = 0;

        const PageHeader *ph = region->root_page;
        while (ph) {
            if (ph->prev != prev) {
                gc_log(region);
                ey_print("label: %s\n", label);
                ey_runtime_panic("gc", "inconsistent gc");
            }
            prev = ph;
            ph = ph->next;
        }
    }
}

void *ey_runtime_gc_alloc(EyGCRegion *region, int block_size, Finaliser finaliser) {
    PageHeader *page = ey_runtime_manual_alloc(sizeof(PageHeader) + block_size);
    if (!page) {
        ey_runtime_panic("ey_runtime_gc_alloc", "Failed to allocate a page");
    }
    *page = (PageHeader){
        .finaliser = finaliser,
        .prev = 0,
        .next = 0,
        .size = block_size,
        .root_count = 0,
    };

    pthread_mutex_lock(&region->mutex);

    gc_check(region, "pre-alloc");

    if (region->root_page) {
        region->root_page->prev = page;
        page->next = region->root_page;
        region->root_page = page;
    } else {
        region->root_page = page;
    }

    region->stats.pages_allocated += 1;
    region->stats.bytes_allocated += block_size;

    void *ptr = gc_ptr_from_page(page);
    memset(ptr, 0, block_size);

    gc_check(region, "alloc");

    pthread_mutex_unlock(&region->mutex);

    return ptr;
}

void *ey_runtime_gc_realloc(EyGCRegion *region, void *ptr, int new_size) {
    pthread_mutex_lock(&region->mutex);

    PageHeader *page = gc_page_from_ptr(ptr);
    const int old_size = page->size;
    if (old_size == new_size) {
        pthread_mutex_unlock(&region->mutex);
        return ptr;
    }

    region->stats.bytes_allocated += new_size - old_size;

    // NB this may move us, so adjust accordingly
    PageHeader *next = page->next;
    PageHeader *previous = page->prev;
    page->size = new_size;
    page = ey_runtime_manual_realloc(page, sizeof(PageHeader) + new_size);
    if (previous) {
        previous->next = page;
    } else {
        region->root_page = page;
    }
    if (next) {
        next->prev = page;
    }
    ptr = gc_ptr_from_page(page);
    if (new_size > old_size) {
        memset(ptr + old_size, 0, new_size - old_size);
    }

    gc_check(region, "realloc");

    pthread_mutex_unlock(&region->mutex);

    return ptr;
}

// the object version
void ey_runtime_gc_forget_root_object(EyGCRegion *region __attribute__((unused)), void *ptr) {
    pthread_mutex_lock(&region->mutex);
    gc_page_from_ptr(ptr)->root_count -= 1;
    pthread_mutex_unlock(&region->mutex);
}

/*
  Recursively mark this page and any other possible pages underneath
 */
static void gc_mark_page(EyGCRegion *region, PageHeader *ph) {
    if (ph->marked) {
        // escaping here will avoid infinite loops
        return;
    }

    ph->marked = k_true;

    const void *base_ptr = gc_ptr_from_page(ph);
    if ((uint64_t)base_ptr % k_pointer_alignment != 0) {
        printf("gc_mark_page: badly aligned page ptr\n");
        exit(1);
    }

    for (int offset = 0; offset <= (ph->size - k_pointer_alignment);
         offset += k_pointer_alignment) {
        void *offset_ptr = *(void **)(base_ptr + offset);
        if (gc_owns_ptr(region, offset_ptr)) {
            gc_mark_page(region, gc_page_from_ptr(offset_ptr));
        }
    }
}

// called locked
static void gc_free_page(EyGCRegion *region, PageHeader *ph) {
    if (ph->finaliser) {
        ph->finaliser(gc_ptr_from_page(ph));
    }

    region->stats.pages_allocated -= 1;
    region->stats.bytes_allocated -= ph->size;

    PageHeader *next = ph->next;

    // unlink the allocation
    if (ph->prev) {
        ph->prev->next = ph->next;
    } else {
        region->root_page = ph->next;
    }
    if (next) {
        next->prev = ph->prev;
    }

    gc_check(region, "free");

    // free the memory (recycling would be an improvement)
    ey_runtime_manual_free(ph);
}

void ey_runtime_gc_collect(EyGCRegion *region) {
    pthread_mutex_lock(&region->mutex);

    // unmark all pages
    PageHeader *ph = region->root_page;
    while (ph) {
        ph->marked = k_false;
        ph = ph->next;
    }

    // mark all roots
    ph = region->root_page;
    while (ph) {
        if (ph->root_count) {
            gc_mark_page(region, ph);
        }
        ph = ph->next;
    }

    // mark all stack roots
    for (int i = 0; i < region->pointers_allocated; i += 1) {
        EyStackPointer *p = region->pointers + i;
        if (!p->in_use) {
            continue;
        }

        void *ptr = *(void **)p->pointer_to_pointer;
        if (gc_owns_ptr(region, ptr)) {
            gc_mark_page(region, gc_page_from_ptr(ptr));
        }
    }

    // sweep unmarked pages
    ph = region->root_page;
    while (ph) {
        PageHeader *this_page = ph;
        ph = ph->next;

        if (!this_page->marked) {
            // unmarked, do delete
            gc_free_page(region, this_page);
        }
    }

    pthread_mutex_unlock(&region->mutex);
}

void ey_runtime_gc_free(EyGCRegion *region) {
    ey_runtime_gc_collect(region);
    pthread_mutex_destroy(&region->mutex);
    ey_runtime_manual_free(region->pointers);
    ey_runtime_manual_free(region);
}

void *ey_runtime_manual_alloc(EyInteger size) {
    return malloc(size);
}

void ey_runtime_manual_free(void *ptr) {
    free(ptr);
}

void *ey_runtime_manual_realloc(void *ptr, EyInteger size) {
    return realloc(ptr, size);
}

// this is the  version that remembers the object itself
void ey_runtime_gc_remember_root_object(EyGCRegion *region __attribute__((unused)), void *ptr) {
    pthread_mutex_lock(&region->mutex);
    gc_page_from_ptr(ptr)->root_count += 1;
    pthread_mutex_unlock(&region->mutex);
}

void ey_runtime_gc_remember_root_pointer(EyGCRegion *region, const void *ptr) {
    pthread_mutex_lock(&region->mutex);

    int pi = -1;
    for (int i = 0; i < region->pointers_allocated; i += 1) {
        if (!region->pointers[i].in_use) {
            pi = i;
            break;
        }
    }

    if (pi < 0) {
        pi = region->pointers_allocated;
        region->pointers_allocated += 1;
        region->pointers = ey_runtime_manual_realloc(
            region->pointers, sizeof(EyStackPointer) * region->pointers_allocated);
    }

    EyStackPointer *p = region->pointers + pi;
    p->in_use = k_true;
    p->pointer_to_pointer = ptr;

    pthread_mutex_unlock(&region->mutex);
}

void ey_runtime_gc_forget_root_pointer(EyGCRegion *region, const void *ptr) {
    pthread_mutex_lock(&region->mutex);

    for (int i = 0; i < region->pointers_allocated; i += 1) {
        EyStackPointer *p = region->pointers + i;
        if (p->pointer_to_pointer == ptr) {
            p->in_use = k_false;
            pthread_mutex_unlock(&region->mutex);
            return;
        }
    }

    ey_runtime_panic("ey_runtime_gc_forget_pointer", "The pointer is not found in the stack list");
}
