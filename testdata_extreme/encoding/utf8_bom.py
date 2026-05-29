#!/usr/bin/env python3
# -*- coding: utf-8-sig -*-

def hello_bom(name: str) -> str:
    """UTF-8 BOM 编码测试"""
    return f"Hello, {name}!"

def calculate_bom(a: int, b: int) -> int:
    result = add_bom(a, b)
    return result

def add_bom(x: int, y: int) -> int:
    return x + y
