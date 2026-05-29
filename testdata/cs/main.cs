using System;
using System.Collections.Generic;

class Program
{
    static string Hello(string name)
    {
        string msg = Greet(name);
        return msg;
    }

    static string Greet(string name)
    {
        return "Hello, " + name + "!";
    }

    static int Calculate(int a, int b)
    {
        int result = Add(a, b);
        result = Sub(result, b);
        result = Mul(result, a);
        return result;
    }

    static int Add(int x, int y)
    {
        return x + y;
    }

    static int Sub(int x, int y)
    {
        return x - y;
    }

    static int Mul(int x, int y)
    {
        return x * y;
    }

    Dictionary<string, int> ProcessData(List<string> items)
    {
        var result = new Dictionary<string, int>();
        foreach (var item in items)
        {
            int count = CountItem(item);
            result[item] = count;
        }
        return result;
    }

    int CountItem(string item)
    {
        return item.Length;
    }
}
