// JavaScript: functions named after keywords (NOT VALID in strict mode,
// but valid in non-strict mode for some keywords)
// Note: 'if', 'for', 'return' etc. are reserved keywords and CANNOT be used

// These ARE valid function names in JS:
function delete(x) { return x; }        // 'delete' is a keyword but can be a function name
function void(x) { return undefined; }  // 'void' can be used
function typeof(x) { return typeof x; } // valid
function instanceof(x) { return x instanceof Object; } // valid

// These are NOT valid but test if regex incorrectly matches:
// function if(x) { }    // SyntaxError
// function for(x) { }   // SyntaxError  

// Normal functions
function normalFunc() { return "normal"; }
const normalArrow = () => "normal";

// Built-in overrides
function parseInt(x) { return Number(x); }
function eval(x) { return eval(x); }
