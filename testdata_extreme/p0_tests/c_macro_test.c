// C99 macro that expands to a function definition — scanner cannot see the expanded form
#include <stdio.h>

// Macro that defines a named function
#define DEFINE_FUNC(name, body) \
    void name() {               \
        body                    \
    }

// Using the macro — this creates a real function called "macro_func"
DEFINE_FUNC(macro_func, printf("hello\n"); )

// Another macro with inline body
#define CREATE_HANDLER(prefix) \
    void prefix##_handler() {  \
        printf(#prefix " handled\n"); \
    }

CREATE_HANDLER(http)
CREATE_HANDLER(ws)

// Multi-line macro
#define IMPLEMENT_SETTER(type, field)   \
    void set_##field(type value) {       \
        global_##field = value;          \
    }

IMPLEMENT_SETTER(int, count)
IMPLEMENT_SETTER(char*, name)

// These are real functions not hidden by macros
void direct_func() {
    printf("direct\n");
}

// C preprocessor conditional — only ONE branch is compiled
#ifdef PLATFORM_WIN
void platform_init() {
    printf("Windows init\n");
}
#else
void platform_init() {
    printf("Linux init\n");
}
#endif

// #if 0 block — dead code, should NOT be scanned
#if 0
void dead_code_func() {
    printf("never compiled\n");
}
#endif
