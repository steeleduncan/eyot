/*
  Eyot strings support
 */

#include "eyot-runtime-cpu.h"

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

/*
  Finaliser for heap allocated strings
  NB this should not be used for literal strings
 */
static void finalise_allocated_string(void *ptr) {
    EyStringS *s = ptr;
    if (!s->static_lifetime) {
        ey_runtime_manual_free((void *)s->ptr);
    }
}

static EyString ey_runtime_string_create_blank(EyExecutionContext *ctx) {
    EyStringS *s =
        ey_runtime_gc_alloc(ey_runtime_gc(ctx), sizeof(EyStringS), finalise_allocated_string);

    *s = (EyStringS) {
        .length = 0,
        .ptr = 0,
        .static_lifetime = k_false,
    };

    return s;
}

EyString ey_runtime_string_copy(EyExecutionContext *ctx, EyString s) {
    EyStringS *ns =
        ey_runtime_gc_alloc(ey_runtime_gc(ctx), sizeof(EyStringS), finalise_allocated_string);
    *ns = (EyStringS){
        .length = s->length,
        .ptr = ey_runtime_manual_alloc(s->length),
        .static_lifetime = k_false,
    };
    memcpy(ns->ptr, s->ptr, s->length);
    return ns;
}

EyString ey_runtime_string_assign(EyExecutionContext *ctx, EyString s) {
    if (s->static_lifetime) {
        return ey_runtime_string_copy(ctx, s);
    } else {
        return s;
    }
}

EyString ey_runtime_string_join(EyExecutionContext *ctx, EyString lhs, EyString rhs) {
    const int length = lhs->length + rhs->length;
    EyStringS *s =
        ey_runtime_gc_alloc(ey_runtime_gc(ctx), sizeof(EyStringS), finalise_allocated_string);
    *s = (EyStringS){
        .length = length,
        .ptr = ey_runtime_manual_alloc(length),
        .static_lifetime = k_false,
    };
    memcpy(s->ptr, lhs->ptr, lhs->length);
    memcpy(s->ptr + lhs->length, rhs->ptr, rhs->length);
    return s;
}

// the length of a utf8 chunk
static int get_utf8_length(const char *p) {
    int l = 0;
    const int len = strlen(p);

    const unsigned char *pp = (const unsigned char *)p;

    for (int i = 0; i < len; i += 1) {
        if (pp[i] >= 0x80 && pp[i] < 0xC0) {
            continue;
        }

        l += 1;
    }

    return l;
}

// Stops at any null characters.
int decode_code_point(char **s) {
    int k = **s ? __builtin_clz(~(**s << 24)) : 0;  // Count # of leading 1 bits.
    int mask = (1 << (8 - k)) - 1;  // All 1's with k leading 0's.
    int value = **s & mask;
    for (++(*s), --k; k > 0 && **s; --k, ++(*s)) {  // Note that k = #total bytes, or 0.
        value <<= 6;
        value += (**s & 0x3F);
    }
    return value;
}

EyString ey_runtime_string_use_literal(EyExecutionContext *ctx, EyString literal) {
    if (literal->static_lifetime) {
        return ey_runtime_string_copy(ctx, literal);
    } else {
        return literal;
    }
}

EyString ey_runtime_string_create_literal(EyExecutionContext *ctx, const char *literal) {
    const int usv_count = get_utf8_length(literal);

    EyStringS *s =
        ey_runtime_gc_alloc(ey_runtime_gc(ctx), sizeof(EyStringS), finalise_allocated_string);
    *s = (EyStringS){
        .length = usv_count * 4,
        .ptr = ey_runtime_manual_alloc(4 * usv_count),
        .static_lifetime = k_false,
    };

    char *sp = (char *)literal;
    for (int i = 0; i < usv_count; i += 1) {
        ((uint32_t *)s->ptr)[i] = decode_code_point(&sp);
    }

    return s;
}

EyInteger ey_runtime_string_character_length(EyExecutionContext *ctx __attribute__((unused)),
                                             EyString s) {
    return s->length / 4;
}

EyCharacter ey_runtime_string_get_character(EyExecutionContext *ctx __attribute__((unused)),
                                            EyString s, int position) {
    return (EyCharacter)((uint32_t *)s->ptr)[position];
}

void ey_runtime_string_set_character(EyExecutionContext *ctx __attribute__((unused)),
                                     EyString s, int position, EyCharacter c) {
    void *ptr = &(((uint32_t *)s->ptr)[position]);
    *(EyCharacter *)ptr = c;
}

EyString ey_runtime_string_resize(EyExecutionContext *ctx, EyString s, EyInteger l) {
    s = ey_runtime_string_assign(ctx, s);

    if (l * 4 == s->length) {
        return s;
    }

    s->ptr = ey_runtime_manual_realloc(s->ptr, l * 4);
    EyCharacter *cs = s->ptr;
    for (int i = s->length / 4; i < l; i += 1) {
        cs[i] = ' ';
    }
    s->length = l * 4;
    return s;
}

EyBoolean ey_runtime_string_equality(EyExecutionContext *ctx __attribute__((unused)), EyString lhs,
                                     EyString rhs) {
    if (lhs == rhs) {
        return k_true;
    }

    const int lhs_length = ey_runtime_string_character_length(ctx, lhs);

    if (lhs_length != ey_runtime_string_character_length(ctx, rhs)) {
        return k_false;
    }

    const char *lp = lhs->ptr, *rp = rhs->ptr;
    for (EyInteger i = 0; i < lhs->length; i += 1) {
        if (lp[i] != rp[i]) {
            return k_false;
        }
    }

    return k_true;
}

const char *ey_runtime_string_create_c_string(EyString eys) {
    int i = 0;
    char *blk = ey_runtime_manual_alloc(eys->length * 6 + 1);

    for (int ind = 0; ind < eys->length / 4; ind += 1) {
        EyUint32 code = ((EyCharacter *)eys->ptr)[ind];
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

        while (val_index--) {
            blk[i] = val[val_index];
            i += 1;
        }
    }

    blk[i] = 0;
    return blk;
}

EyString ey_runtime_string_get(EyExecutionContext *ctx __attribute__((unused)),
                               EyInteger string_index) {
    return &ey_string_pool_raw[string_index];
}
