// === 极限测试: JS 特殊字符干扰 ===

// --- 测试1: 表情符号 ---
function emojiFunc() {
    // 🚀🔥⭐ 表情符在注释中
    const msg = "Hello 😊 World 🌍";
    return msg;
}

// --- 测试2: 零宽字符 ---
function zeroWidthTest() {
    // 零宽空格\u200B\u200C不应影响
    const s = "test\u200Bfunc";
    return s;
}

// --- 测试3: 中文函数名（JS 支持）---
function 计算总计(数值列表) {
    return 数值列表.reduce((a, b) => a + b, 0);
}

function 获取用户名(用户ID) {
    const 用户映射 = {1: "张三", 2: "李四"};
    return 用户映射[用户ID] || "未知";
}

// --- 测试4: 日文函数名 ---
function データ処理(値) {
    return 値 * 2;
}

// --- 测试5: 韩文函数名 ---
function 데이터_처리(값) {
    return 값 * 2;
}

// --- 测试6: 箭头函数中的特殊字符 ---
const arrowWithEmoji = (msg) => {
    // 箭头函数中的emoji: 🎯✅
    return `Message: ${msg}`;
};

// --- 测试7: 模板字符串中的函数定义（不应匹配）---
const template = `
function inTemplate1() {
    return "should be ignored";
}
`;

// --- 测试8: 全角字符和半角混合 ---
function ｆｕｎｃ() {  // 全角函数名（实际有效）
    return "fullwidth";
}

// --- 测试9: RTL 覆盖字符 ---
function rtlOverride() {
    // 下面的 RTL 覆盖字符不应破坏解析
    const s = "hello\u202Eworld";
    return s;
}

// --- 测试10: 控制字符 ---
function controlChars() {
    const special = {
        tab: "\t",
        null: "\x00",
        bell: "\x07",
        escape: "\x1b"
    };
    return special;
}

// 真正的函数
function realJsExtremeFunc() {
    return "done";
}
