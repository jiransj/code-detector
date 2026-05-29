// ============================================================================
// 超级测试: JavaScript 局部/全局函数混杂 + 注释干扰
// JS 支持函数内嵌套函数声明、箭头函数、class 方法等多种形式
// ============================================================================

// --- 测试1: 全局函数声明 ---
function globalFuncA(x) {
    return x + 1;
}

const globalFuncB = (y) => y * 2;

// --- 测试2: 函数内部嵌套函数声明 ---
function outerWithLocals(name) {
    // 函数内部声明的命名函数（局部函数）
    function innerGreet() {
        return `Hello, ${name}!`;
    }
    
    function innerBye() {
        return `Goodbye, ${name}!`;
    }
    
    const innerArrow = (msg) => {
        return `[${name}] ${msg}`;
    };
    
    return innerGreet(), innerBye();
}

// --- 测试3: 多层嵌套 ---
function topLevel(msg) {
    function middleLevel(prefix) {
        function bottomLevel(suffix) {
            return `${prefix}: ${msg} ${suffix}`;
        }
        return bottomLevel("!!!");
    }
    return middleLevel("INFO");
}

// --- 测试4: 闭包返回 ---
function makeCounter(base) {
    let count = base;
    return function() {
        return count++;
    };
}

function makeMultiplier(factor) {
    return function(x) {
        return x * factor;
    };
}

// --- 测试5: 块注释中的完整函数 ---
/*
function blockCommentedA() {
    return "should NOT be detected";
}

const blockCommentedB = () => {
    return "should NOT be detected";
};

class BlockCommentedClass {
    method() {
        return "should NOT be detected";
    }
}
*/

// --- 测试6: 行注释中的函数 ---
// function lineCommentedA() {
//     return "should NOT be detected";
// }
//
// const lineCommentedB = () => {
//     return "should NOT be detected";
// };

// --- 测试7: 模板字符串中的函数 ---
const TEMPLATE_CODE = `
function insideTemplate1() {
    return "should NOT be detected";
}

const insideTemplate2 = () => {
    return "should NOT be detected";
};
`;

// --- 测试8: 字符串中的函数 ---
const STRING_CODE = "\
function insideString1() {\
    return 'should NOT be detected';\
}";

// --- 测试9: Class 中的方法与内部函数 ---
class SuperAdvanced {
    constructor(value) {
        this.value = value;
        // 构造器中的局部函数
        function initHelper(v) {
            return v * 2;
        }
        this.value = initHelper(value);
    }
    
    methodA() {
        // 方法中的局部函数
        function localHelper() {
            return this.value + 1;
        }
        return localHelper.call(this);
    }
    
    methodB() {
        const arrowHelper = () => {
            return this.value * 2;
        };
        return arrowHelper();
    }
    
    static staticMethod(x) {
        function localToStatic(y) {
            return x + y;
        }
        return localToStatic(1);
    }
    
    // getter
    get computed() {
        return this.value * 10;
    }
    
    // setter
    set computed(val) {
        this.value = val / 10;
    }
    
    // generator method
    *genMethod() {
        yield 1;
        yield 2;
    }
}

// --- 测试10: 条件分支中的函数 ---
function conditionalBuilder(flag) {
    if (flag) {
        function branchA() {
            return "branch A";
        }
        return branchA();
    } else {
        function branchB() {
            return "branch B";
        }
        return branchB();
    }
}

// --- 测试11: 数组/对象中的函数 ---
const handlerMap = {
    handleA: function(data) {
        return data.a;
    },
    handleB(data) {
        return data.b;
    },
    handleC: (data) => data.c,
};

// --- 测试12: IIFE 中的函数 ---
const IIFE_RESULT = (function() {
    function iifeInternal() {
        return "iife internal";
    }
    return iifeInternal();
})();

// --- 测试13: eval 中的函数 ---
eval("function evalCreated() { return 'eval'; }");

// --- 测试14: 异步函数与生成器 ---
async function asyncGlobal() {
    return await Promise.resolve(42);
}

const asyncArrowGlobal = async () => {
    return await Promise.resolve(84);
};

async function* asyncGenGlobal() {
    yield await Promise.resolve(1);
}

// --- 测试15: 大量真实函数模拟项目 ---
function loginUser(username, password) {
    return validateCredentials(username, password);
}

function validateCredentials(user, pass) {
    return user === "admin" && pass === "secret";
}

function logoutUser(sessionId) {
    clearSession(sessionId);
}

function clearSession(id) {
    console.log("clearing", id);
}

// --- 测试16: 注释陷阱 - 看起来像代码的注释块 ---
/* 以下代码块是注释，但看起来像真实代码
const api = require('express');
const app = api();

function setupRoutes() {
    app.get('/api/users', function(req, res) {
        res.json({users: []});
    });
    
    app.post('/api/users', function(req, res) {
        const data = req.body;
        saveUser(data);
    });
}

function saveUser(data) {
    return database.insert('users', data);
}

function initServer() {
    setupRoutes();
    app.listen(3000);
}
*/

// --- 测试17: 另一个注释陷阱 - 多语言混合注释 ---
/* 
Python code in JS comment:
def helper():
    return "helper"

Java code in JS comment:
public void helper() {
    return "helper";
}

Go code in JS comment:
func Helper() string {
    return "helper"
}
*/

// --- 测试18: 箭头函数多种形态 ---
const noArgArrow = () => 42;
const singleArgArrow = x => x * 2;
const multiArgArrow = (a, b) => { return a + b; };
const objectReturn = () => ({ key: "value" });
const arrayReturn = () => [1, 2, 3];

// --- 测试19: 函数名与关键字相似的函数 ---
function _if(x) { return x; }
function _for(items) { return items.length; }
function _while(cond) { return cond ? 1 : 0; }
function _return(x) { return x; }
function _switch(val) { return val; }

// --- 测试20: 真实最终函数 ---
function ultimateRealJsFunc() {
    return "I am real";
}
