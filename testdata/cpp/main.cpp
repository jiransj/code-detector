#include <string>
#include <map>
#include <vector>

std::string hello(std::string name) {
    std::string msg = greet(name);
    return msg;
}

std::string greet(std::string name) {
    return "Hello, " + name + "!";
}

int calculate(int a, int b) {
    int result = add(a, b);
    result = sub(result, b);
    result = mul(result, a);
    return result;
}

int add(int x, int y) {
    return x + y;
}

int sub(int x, int y) {
    return x - y;
}

int mul(int x, int y) {
    return x * y;
}
