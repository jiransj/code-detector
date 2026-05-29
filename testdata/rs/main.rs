fn hello(name: &str) -> String {
    let msg = greet(name);
    msg
}

fn greet(name: &str) -> String {
    format!("Hello, {}!", name)
}

fn calculate(a: i32, b: i32) -> i32 {
    let result = add(a, b);
    let result = sub(result, b);
    let result = mul(result, a);
    result
}

fn add(x: i32, y: i32) -> i32 {
    x + y
}

fn sub(x: i32, y: i32) -> i32 {
    x - y
}

fn mul(x: i32, y: i32) -> i32 {
    x * y
}

fn process_data(items: Vec<&str>) -> std::collections::HashMap<String, usize> {
    let mut result = std::collections::HashMap::new();
    for item in items {
        let count = count_item(item);
        result.insert(item.to_string(), count);
    }
    result
}

fn count_item(item: &str) -> usize {
    item.len()
}
