// JavaScript 测试文件
function hello(name) {
    const msg = greet(name);
    return msg;
}

function greet(name) {
    return `Hello, ${name}!`;
}

function calculate(a, b) {
    let result = add(a, b);
    result = sub(result, b);
    result = mul(result, a);
    return result;
}

const add = (x, y) => x + y;
const sub = (x, y) => x - y;
const mul = function(x, y) {
    return x * y;
};

async function processData(items) {
    const result = {};
    for (const item of items) {
        const count = await countItem(item);
        result[item] = count;
    }
    return result;
}

function countItem(item) {
    return item.length;
}

class Calculator {
    constructor(initial = 0) {
        this.value = initial;
    }
    
    add(amount) {
        this.value = this.value + amount;
        return this.value;
    }
    
    subtract(amount) {
        this.value = this.value - amount;
        return this.value;
    }
    
    getValue() {
        return this.value;
    }
}
