# -*- coding: utf-8 -*-
# Python triple-quote nesting traps

# === Trap 1: docstring within a triple-quoted string ===
def trap1():
    """外部 docstring"""
    code = '''
def fake_inner():
    """内部还有 docstring"""
    pass
'''
    return code

# === Trap 2: Mixed triple quotes (''' inside """) ===
def trap2():
    code = """
def func_a():
    ''' 这是单引号三引号 '''
    pass
"""
    return code

# === Trap 3: Triple quotes inside triple quotes (same type) ===
def trap3():
    code = """开头"""  # 这里提前结束了！
    # 上面的 """开头""" 会被解释器认为是 空字符串 "开头" 再跟一个空字符串
    # 但实际上 """ 开头 """ 是 一个空字符串 """ 然后 " 开头 " 然后 """
    # 下面的代码实际上是真实代码
    def real_after_trap():
        return "still real"
    return real_after_trap()

# === Trap 4: Consecutive triple quotes ===
def trap4():
    code = """""" 
    # 上面的 """""" 是 空字符串 "" 拼接 空字符串 "" 
    # 还是 一个三引号字符串 """"""?
    # Python 中 """""" 是一个空的三引号字符串！
    return "trap4 ok"

# === Trap 5: Five quotes ===
def trap5():
    s = """"""
    # 5个引号: """"" = 一个三引号字符串 """ 之后跟一个 " 
    # 但 Python 词法分析器优先匹配 """
    # 所以 """""" 是 两个 空三引号字符串
    # 而 """"" 是 一个三引号字符串 """ 加 一个双引号 "
    return "trap5 ok"

# === Trap 6: Single backslash at end of line inside triple-quote ===
def trap6():
    code = """\
这是一行\
第二行\
第三行"""
    return code

# === Real functions that should be detected ===
def real_function_a():
    return "a"

def real_function_b():
    return "b"
