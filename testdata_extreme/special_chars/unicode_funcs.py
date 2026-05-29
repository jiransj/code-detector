# -*- coding: utf-8 -*-
"""极限测试: 特殊字符干扰 - Unicode 函数名和特殊字符"""

# === 测试1: 中文函数名 ===
def 计算总计(数值列表):
    """计算列表中所有数值的总和"""
    return sum(数值列表)

def 获取用户名(用户ID):
    用户映射 = {1: "张三", 2: "李四", 3: "王五"}
    return 用户映射.get(用户ID, "未知用户")

def 处理_请求_v2(url, 超时时间=30):
    """混合中英文和下划线的函数名"""
    return f"处理 {url} 超时={超时时间}"

# === 测试2: 日文函数名 ===
def データ処理(値):
    """日文函数名测试"""
    return 値 * 2

def ユーザー情報取得(id):
    return {"id": id, "name": "テスト"}

# === 测试3: 韩文函数名 ===
def 데이터_처리(값):
    """韩文函数名测试"""
    return 값 * 2

# === 测试4: 表情符号在字符串和注释中 ===
def emoji_handler():
    """处理包含😀emoji😎的文档字符串"""
    # 注释中的emoji: 🚀🔥⭐
    data = "函数调用测试 🎯✅"
    return data

# === 测试5: 零宽字符注入 ===
def zero_width_test():
    """零宽字符测试"""
    # 以下字符串包含零宽空格 (U+200B) 和零宽非连接符 (U+200C)
    # 这些不应该影响函数检测
    s = "test\u200Bstring\u200Cwith\u200Dzeros"
    return s

# === 测试6: RTL 覆盖字符 ===
def rtl_test():
    """RTL 覆盖字符测试 \u202E"""
    s = "hello\u202Eworld"
    return s

# === 测试7: 函数名包含特殊 Unicode 字符 ===
def café_reservation():
    """带重音符号的函数名"""
    return "café"

def naïve_implementation():
    return "naïve"

def façàde_handler():
    return "façade"

# === 测试8: 数学符号在注释中 ===
def math_func():
    """数学符号测试: ∑ ∫ π ≈ ≠ ≤ ≥ ∞"""
    # 这也是注释: α β γ δ ε θ λ μ
    return 42

# === 测试9: 全角字符 ===
def 全角テスト():
    """全角字符函数名"""
    return "全角"

# === 测试10: 字符串中的各种干扰字符 ===
def interference_test():
    special = {
        "tab_in_string": "	",  # 实际tab字符
        "null_char": "\x00",     # 空字符
        "bell": "\x07",         # 响铃
        "escape": "\x1b",       # ESC
        "unicode_control": "\u009C",  # 控制字符
    }
    return special

# === 测试11: 伪装成注释的代码 ===
def real_function():
    # 下面的行看起来像函数定义，但实际在注释中
    # def fake_in_comment(): pass
    # class FakeInComment: pass
    return "real"

# === 测试12: 带有非常规缩进的函数 ===
def weird_indent():
    """使用混合 tab 和空格的函数（视觉上可能混乱）"""
	  # 这行使用 tab 缩进
    return "ok"

# === 测试13: 带有 Unicode 转义序列的字符串 ===
def unicode_escape():
    s = "\u0066\u0075\u006e\u0063"  # "func" 的 unicode 转义
    return s

# === 测试14: 函数名以数字结尾 ===
def handler_1():
    return 1

def handler_2():
    return 2

def version_2_0():
    return "2.0"

# === 测试15: 极短函数名 ===
def f():
    return 1

def gx():
    return 2

# === 测试16: 同一行多个函数定义（不应匹配第二个） ===
def a(): return 1  # def b(): return 2  <- 注释中的不应匹配

# === 测试17: 字符串中的字符串中的函数 ===
def nested_string_test():
    # "def inside_double(): pass" 在字符串中，不应匹配
    code = '''def inside_triple():
    pass
'''
    return code

# === 测试18: 带有多种引号的函数 ===
def quote_test():
    single = 'single'
    double = "double"
    triple_single = '''triple'''
    triple_double = """triple"""
    return single + double

# 真正的函数
def real_ultimate_test():
    return "test complete"
