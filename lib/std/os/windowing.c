#include <stdio.h>
#include <SDL2/SDL.h>
#include <GL/gl.h>
#include "eyot-runtime-common.h"

static SDL_Window *_window = 0;
static SDL_GLContext _context = 0;

EyBoolean sdleyot_init(EyExecutionContext *ctx, int w, int h) {
    if (SDL_Init(SDL_INIT_VIDEO) < 0) {
        fprintf(stderr, "sdleyot_init: SDL initialisation failed\n");
        return k_false;
    }

    _window = SDL_CreateWindow("eyot", SDL_WINDOWPOS_CENTERED, SDL_WINDOWPOS_CENTERED, w, h, SDL_WINDOW_SHOWN | SDL_WINDOW_OPENGL);
    if (_window == 0) {
        fprintf(stderr, "sdleyot_init: Window creation failed\n");
        sdleyot_teardown();
        return k_false;
    }

    SDL_GL_SetAttribute(SDL_GL_CONTEXT_MAJOR_VERSION, 3);
    SDL_GL_SetAttribute(SDL_GL_CONTEXT_MINOR_VERSION, 1);
    SDL_GL_SetAttribute(SDL_GL_CONTEXT_PROFILE_MASK, SDL_GL_CONTEXT_PROFILE_CORE);

    _context = SDL_GL_CreateContext(_window);
    if (_context == NULL) {
        fprintf(stderr, "sdleyot_init: GL Context creation failed\n");
        sdleyot_teardown();
        return k_false;
    }

    return k_true;
}

void sdleyot_clear(EyExecutionContext *ctx, int r, int g, int b) {
    glClearColor(r, g, b, 255);
    glClear(GL_COLOR_BUFFER_BIT);
    glFlush();
    SDL_GL_SwapWindow(_window);
}

void sdleyot_teardown(EyExecutionContext *ctx) {
    if (_window) {
        SDL_DestroyWindow(_window);
        _window = 0;
    }
}
                             
