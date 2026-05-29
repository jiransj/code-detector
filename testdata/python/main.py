def hello(name):
    """一个简单的问候函数"""
    msg = greet(name)
    return msg

def greet(name):
    """被 hello 调用的函数"""
    return f"Hello, {name}!"

def calculate(a, b):
    """测试多个参数"""
    result = add(a, b)
    result = sub(result, b)
    result = mul(result, a)
    return result

def add(x, y):
    return x + y

def sub(x, y):
    return x - y

def mul(x, y):
    return x * y

def process_data(items):
    """带依赖的复杂函数"""
    result = {}
    for item in items:
        count = count_item(item)
        result[item] = count
    return result

def count_item(item):
    return len(item)

class Calculator:
    """一个计算器类"""
    def __init__(self, initial=0):
        self.value = initial
    
    def add(self, amount):
        self.value = self.value + amount
        return self.value
    
    def subtract(self, amount):
        self.value = self.value - amount
        return self.value
    
    def get_value(self):
        return self.value
