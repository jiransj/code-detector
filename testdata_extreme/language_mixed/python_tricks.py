# -*- coding: utf-8 -*-
"""极限测试: Python 文件的各类边界情况"""

# === 测试1: 注释中的伪函数定义 ===
# def commented_out_func():
#     pass
# 
# class CommentedClass:
#     def method(self):
#         pass

"""
块注释中的伪函数:
def fake_block_func():
    pass
    
class FakeClass:
    def fake_method(self):
        pass
"""

# === 测试2: 字符串中的伪函数 ===
GO_CODE_SAMPLE = """
package main
func main() {
    fmt.Println("hello")
}
"""

JS_CODE_SAMPLE = '''
function jsFunc() {
    return "this is js in python string";
}
'''

JAVA_CODE = """public class Test {
    public void execute() {
        System.out.println("java in python");
    }
}
"""

# === 测试3: 装饰器函数 ===
def decorator(func):
    def wrapper(*args, **kwargs):
        print("before")
        result = func(*args, **kwargs)
        print("after")
        return result
    return wrapper

@decorator
def decorated_func(name):
    """这是一个被装饰的函数"""
    return f"Hello, {name}!"

@decorator
@decorator
def double_decorated(a, b):
    return a + b

# === 测试4: 类型提示函数 ===
def typed_func(name: str, age: int) -> str:
    return f"{name} is {age} years old"

def complex_type(items: list[int], mapping: dict[str, int]) -> list[str]:
    result = []
    for k, v in mapping.items():
        result.append(f"{k}: {v}")
    return result

# === 测试5: 生成器函数 ===
def generator_func(n: int):
    """生成器函数"""
    for i in range(n):
        yield i * i

def infinite_gen():
    """无限生成器"""
    n = 0
    while True:
        yield n
        n += 1

# === 测试6: 异步函数 ===
async def async_func(url: str):
    import asyncio
    await asyncio.sleep(1)
    return f"fetched {url}"

async def async_processor(items: list[int]):
    results = []
    for item in items:
        result = await process_item(item)
        results.append(result)
    return results

async def process_item(item: int):
    return item * 2

# === 测试7: 类方法 ===
class ExtremeTest:
    class_var = 42
    
    def __init__(self, name: str):
        self.name = name
        self._data = {}
    
    def instance_method(self, x: int) -> int:
        return x * self.class_var
    
    @classmethod
    def class_method(cls):
        return cls.class_var
    
    @staticmethod
    def static_method(x: int, y: int) -> int:
        return x + y
    
    @property
    def name_property(self):
        return self._name
    
    @name_property.setter
    def name_property(self, value):
        self._name = value
    
    def _private_method(self):
        return "private"
    
    def __dunder_method__(self):
        return "dunder"

# === 测试8: 嵌套函数 ===
def outer_func(x: int):
    """外层函数"""
    def inner_func(y: int):
        """内层函数"""
        def deepest(z: int):
            return x + y + z
        return deepest(y)
    return inner_func(x)

# === 测试9: lambda（不应匹配）===
lambda_func = lambda x: x * 2  # 不应被识别为函数
sorted([3, 1, 2], key=lambda x: -x)  # lambda 在参数中

# === 测试10: eval/exec 中的函数定义 ===
eval("def eval_func(): return 42")
exec("def exec_func(): return 'exec'")

# === 测试11: 非常长的函数名 ===
def this_is_a_very_very_long_function_name_that_should_still_be_detected_correctly_by_the_scanner():
    return "long name"

# === 测试12: 中文函数名 ===
def 计算总计(数值列表):
    return sum(数值列表)

def 获取用户名(用户ID):
    用户映射 = {1: "张三", 2: "李四"}
    return 用户映射.get(用户ID, "未知")

# === 测试13: 单行函数定义 ===
def short_func(): return 42

# === 测试14: 只有文档字符串的函数 ===
def docstring_only():
    """This function has only a docstring."""
    pass

# === 测试15: 条件分支中的函数定义（实际有效） ===
if True:
    def conditional_func():
        return "conditional"

# === 测试16: 字符串中包含大量函数模式 ===
MULTILINE_FAKE = (
    "def func1():\n"
    "    pass\n"
    "def func2():\n"
    "    pass\n"
    "class Cls:\n"
    "    def method(self): pass\n"
)

# 真正的函数
def realPythonFunc():
    add = lambda x, y: x + y  # lambda 不应识别
    return add(1, 2)
