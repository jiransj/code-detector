// C comment continuation with backslash-newline
#include <stdio.h>

// The following function definition is commented out...
// But there is a backslash at the end -> the next line is also part of the comment!

// func_not_seen() { \
//     printf("hidden\n"); \
// }

// The above is ALL commented out. But what if the backslash is NOT at end of line?

int visible_func() {
    // The line below has a \ at the end (line continuation)
    // This continues to the next line in C preprocessor
    prin\
tf("line continued\n");
    return 42;
}

// Single-line comment with backslash continuation (GCC extension or preprocessor)
// The backslash at end of line 1 continues to line 2:
// this is all still a comment \
// this line is also a comment (continuation)
int after_comment() {
    return 1;
}

// Real functions
int real_a() { return 1; }
int real_b() { return 2; }
