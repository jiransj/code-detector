def hello(name)
    msg = greet(name)
    msg
end

def greet(name)
    "Hello, #{name}!"
end

def calculate(a, b)
    result = add(a, b)
    result = sub(result, b)
    result = mul(result, a)
    result
end

def add(x, y)
    x + y
end

def sub(x, y)
    x - y
end

def mul(x, y)
    x * y
end

def process_data(items)
    result = {}
    items.each do |item|
        count = count_item(item)
        result[item] = count
    end
    result
end

def count_item(item)
    item.length
end
