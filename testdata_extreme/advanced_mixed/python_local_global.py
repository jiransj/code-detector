# -*- coding: utf-8 -*-
# ============================================================================
# 超级测试: Python 局部/全局函数混杂 + 注释干扰
# Python 支持真正的嵌套命名函数（局部函数），这是测试重点
# ============================================================================

# --- 测试1: 全局函数 ---
def global_func_a(x):
    return x + 1

def global_func_b(y):
    return y * 2

# --- 测试2: 嵌套命名函数（局部函数 vs 全局函数）---
def outer_with_locals(name):
    """外层函数，内部定义了多个局部函数"""
    
    def inner_greet():
        """内部局部函数1：生成问候语"""
        return f"Hello, {name}!"
    
    def inner_bye():
        """内部局部函数2：生成告别语"""
        return f"Goodbye, {name}!"
    
    def inner_format(msg):
        """内部局部函数3：格式化消息"""
        return f"[{name}] {msg}"
    
    return inner_greet(), inner_bye()

# --- 测试3: 多层嵌套局部函数 ---
def top_level(msg):
    """顶层函数"""
    
    def middle_level(prefix):
        """中层函数"""
        
        def bottom_level(suffix):
            """底层函数 - 最深层的嵌套"""
            return f"{prefix}: {msg} {suffix}"
        
        return bottom_level("!!!")
    
    return middle_level("INFO")

# --- 测试4: 条件分支中的函数定义 ---
def conditional_func_builder(flag):
    if flag:
        def local_func_a():
            return "branch_a"
        return local_func_a
    else:
        def local_func_b():
            return "branch_b"
        return local_func_b

# --- 测试5: 循环中的函数定义 ---
def loop_func_builder():
    funcs = []
    for i in range(3):
        def make_func(n):
            def inner():
                return n * 10
            return inner
        funcs.append(make_func(i))
    return funcs

# --- 测试6: 类内部的函数 vs 方法 ---
class AdvancedClass:
    """包含方法、静态方法、类方法、属性的类"""
    
    class_var = 42
    
    def __init__(self, value):
        self.value = value
        # 实例方法内部的局部函数
        def init_helper(v):
            return v * 2
        self.value = init_helper(value)
    
    def method_a(self):
        """实例方法"""
        # 方法内部的局部函数
        def local_helper():
            return self.value + 1
        return local_helper()
    
    @classmethod
    def class_method(cls):
        """类方法"""
        def local_to_class():
            return cls.class_var
        return local_to_class()
    
    @staticmethod
    def static_method(x):
        """静态方法"""
        def local_to_static(y):
            return x + y
        return local_to_static(1)
    
    @property
    def computed_property(self):
        """属性方法"""
        return self.value * 2
    
    # 内部类
    class InnerClass:
        def inner_method(self):
            return "inner"

# --- 测试7: 注释中的完整函数（块注释）---
"""
def block_commented_func_a():
    return "should NOT be detected"

def block_commented_func_b(x):
    return x ** 2

class BlockCommentedClass:
    def method(self):
        pass
"""

# --- 测试8: 注释中的函数（行注释）---
# def line_commented_func_a():
#     return "should NOT be detected"
#
# def line_commented_func_b():
#     return "also NOT detected"

# --- 测试9: 字符串中的完整函数 ---
GO_CODE_SAMPLE = """
def fake_in_string():
    return "this is in a string, should NOT be detected"

def another_fake():
    return "also fake"
"""

# --- 测试10: docstring 中的函数定义 ---
def func_with_tricky_docstring():
    """这个 docstring 包含了看起来像函数定义的内容
    
    下面看似 def 但不是真实的函数定义:
    
    def fake_inside_docstring():
        pass
    
    def another_fake_inside():
        return 42
    
    但这些只是文档字符串的一部分
    """
    return "real"

# --- 测试11: eval/exec 中的函数定义 ---
def dynamic_func_creator():
    """通过 exec 创建的函数不应被静态扫描到"""
    code = """
def dynamically_created():
    return "dynamic"
"""
    exec(code)
    # 通过 exec 定义的函数在运行时才存在，静态扫描不应发现

# --- 测试12: 同文件大量函数，模拟真实项目 ---
def user_login(username, password):
    return _validate_credentials(username, password)

def _validate_credentials(user, passwd):
    return user == "admin" and passwd == "secret"

def user_logout(session_id):
    _clear_session(session_id)

def _clear_session(sid):
    pass

def get_user_profile(uid):
    return _fetch_profile(uid)

def _fetch_profile(uid):
    return f"profile_{uid}"

# --- 测试13: 装饰器工厂（返回函数的函数）---
def decorator_factory(prefix):
    """装饰器工厂 - 返回一个装饰器"""
    def actual_decorator(func):
        def wrapper(*args, **kwargs):
            print(f"{prefix}: calling {func.__name__}")
            return func(*args, **kwargs)
        return wrapper
    return actual_decorator

@decorator_factory("TEST")
def decorated_greeting(name):
    return f"Hello, {name}!"

# --- 测试14: lambda 不应被检测 ---
lambda_square = lambda x: x * x
lambda_cube = lambda x: x * x * x
sorted([3, 1, 2], key=lambda x: -x)

# --- 测试15: 在条件为真的分支中定义函数 ---
if __name__ == "__main__":
    def main_func():
        return "main"
    
    def main_helper():
        return "helper"

# --- 测试16: try/except 中的函数定义 ---
def try_block_func():
    try:
        def success_handler():
            return "success"
        return success_handler
    except:
        def error_handler():
            return "error"
        return error_handler

# --- 测试17: with 语句块中的函数定义 ---
def with_block_func():
    class Context:
        def __enter__(self): return self
        def __exit__(self, *args): pass
    
    with Context():
        def inside_with():
            return "inside with"
        return inside_with()

# --- 测试18: 多行字符串中的多语言函数 ---
MULTI_LANG_STRINGS = """\
// Go code in string
func goFunc() string {
    return "go"
}

// Java code in string
public void javaMethod() {
    System.out.println("java");
}
"""

# --- 测试19: 注释中嵌套注释的函数陷阱 ---
def comment_trap_real():
    # 下面的注释包含嵌套函数定义
    # def level1():
    #     def level2():
    #         def level3():
    #             return "deep"
    #         return level3()
    #     return level2()
    return "trap passed"

# --- 测试20: 真实最终函数 ---
def ultimate_real_python_func():
    """这个函数是真正应该被检测到的"""
    return "I am real"
