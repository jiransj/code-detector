// === 极限测试: JavaScript 边界情况 ===

// --- 测试1: 注释中的伪函数 ---
/*
function commentedFunc() {
    return "hidden";
}
*/

// function commentedLine() { return "hidden"; }

// --- 测试2: 字符串中的函数模式 ---
const fakeGoCode = `
func main() {
    fmt.Println("hello")
}
`;

const fakePython = `
def hello():
    print("world")
`;

// --- 测试3: 各种函数定义形式 ---

// 传统函数声明
function traditionalFunc(a, b) {
    return a + b;
}

// 函数表达式赋给变量
const exprFunc = function(a, b) {
    return a + b;
};

// 箭头函数赋给变量
const arrowFunc = (a, b) => {
    return a + b;
};

// 单行箭头函数
const shortArrow = (x) => x * 2;

// 无参数箭头函数
const noParamArrow = () => 42;

// --- 测试4: 异步函数 ---
async function asyncTraditional(url) {
    const response = await fetch(url);
    return response.json();
}

const asyncArrow = async (url) => {
    const data = await fetch(url);
    return data.json();
};

// --- 测试5: 生成器函数 ---
function* generatorFunc() {
    yield 1;
    yield 2;
    yield 3;
}

// --- 测试6: 类方法 ---
class ExtremeClass {
    constructor(name) {
        this.name = name;
        this.#privateField = 42;  // 私有字段
    }
    
    // 实例方法
    instanceMethod(x) {
        return x + this.#privateMethod();
    }
    
    // getter
    get nameUpper() {
        return this.name.toUpperCase();
    }
    
    // setter
    set nameUpper(value) {
        this.name = value.toLowerCase();
    }
    
    // 静态方法
    static staticMethod(x, y) {
        return x + y;
    }
    
    // 私有方法
    #privateMethod() {
        return this.#privateField;
    }
    
    // 异步方法
    async asyncMethod() {
        return await Promise.resolve(42);
    }
    
    // 生成器方法
    *generatorMethod() {
        yield* [1, 2, 3];
    }
}

// --- 测试7: 嵌套函数 ---
function outer() {
    function inner() {
        return "inner";
    }
    
    const innerArrow = () => {
        return "innerArrow";
    };
    
    return inner() + innerArrow();
}

// --- 测试8: 立即执行函数表达式 (IIFE) ---
const result = (function() {
    const hidden = 42;
    return hidden;
})();

const arrowIIFE = (() => {
    return 42;
})();

// --- 测试9: 对象字面量方法 ---
const obj = {
    // 简写方法
    method1(x) {
        return x;
    },
    
    // 传统方法
    method2: function(y) {
        return y;
    },
    
    // 箭头方法
    method3: (z) => z,
    
    // getter
    get data() {
        return this._data;
    },
    
    // setter
    set data(val) {
        this._data = val;
    },
    
    // 异步方法
    async asyncMethod() {
        return await Promise.resolve(1);
    },
    
    // 生成器方法
    *genMethod() {
        yield 1;
    }
};

// --- 测试10: 模板字符串中的函数 ---
const template = `
function inTemplate1() {
    return "should not be detected";
}
function inTemplate2() {
    return "also should not be detected";
}
`;

// --- 测试11: eval 中的函数定义 ---
eval("function evalFunc() { return 'eval'; }");

// --- 测试12: 空函数 ---
function emptyFunc() {}

// --- 测试13: 带默认参数的函数 ---
function defaultParams(a = 1, b = 2, c = a + b) {
    return a + b + c;
}

// --- 测试14: 解构参数 ---
function destructureParams({ name, age }, [first, ...rest]) {
    return `${name}: ${first}`;
}

// --- 测试15: Rest 参数 ---
function restParams(...args) {
    return args.length;
}

// --- 测试16: 非常长的函数名 ---
function this_is_a_very_very_long_function_name_that_should_still_be_detected_xxxxxxxxxxxxxxxxxxxx() {
    return "long";
}

// --- 测试17: 函数名含数字 ---
function handlerV2_0_1(data) {
    return data;
}

// --- 测试18: for/if/while 不应被识别为函数 ---
function testKeywords() {
    // for 是关键字，但下面的不是函数定义
    for (let i = 0; i < 10; i++) {
        // 循环体
    }
    
    if (true) {
        // 条件体
    }
    
    while (false) {
        // 循环体
    }
    
    switch (1) {
        case 1:
            break;
    }
    
    try {
        // try 体
    } catch (e) {
        // catch 体
    }
    
    return true;
}

// 真正的函数
function realJsFunc() {
    return "real";
}
