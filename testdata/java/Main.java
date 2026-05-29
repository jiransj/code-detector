import java.util.*;

public class Main {
    
    public String hello(String name) {
        String msg = greet(name);
        return msg;
    }
    
    private String greet(String name) {
        return "Hello, " + name + "!";
    }
    
    public int calculate(int a, int b) {
        int result = add(a, b);
        result = sub(result, b);
        result = mul(result, a);
        return result;
    }
    
    private int add(int x, int y) {
        return x + y;
    }
    
    private int sub(int x, int y) {
        return x - y;
    }
    
    private int mul(int x, int y) {
        return x * y;
    }
    
    public Map<String, Integer> processData(List<String> items) {
        Map<String, Integer> result = new HashMap<>();
        for (String item : items) {
            int count = countItem(item);
            result.put(item, count);
        }
        return result;
    }
    
    private int countItem(String item) {
        return item.length();
    }
}
