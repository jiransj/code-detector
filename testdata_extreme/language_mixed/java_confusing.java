import java.util.*;
import java.util.function.*;

/**
 * 极限测试: Java 边界情况
 */

public class java_confusing {
    
    // === 测试1: 注释中的伪函数 ===
    /*
    public void commentedMethod() {
        System.out.println("hidden");
    }
    */
    
    // public void commentedLine() { System.out.println("hidden"); }
    
    // === 测试2: 字符串中的函数模式 ===
    String goCode = """
        package main
        func main() {
            fmt.Println("hello")
        }
        """;
    
    String pythonCode = """
        def hello():
            print("world")
        """;
    
    // === 测试3: 各种修饰符组合 ===
    public void publicMethod() {}
    private void privateMethod() {}
    protected void protectedMethod() {}
    static void staticMethod() {}
    final void finalMethod() {}
    abstract void abstractMethod();
    synchronized void syncMethod() {}
    public static void main(String[] args) {}
    public static final void staticFinalMethod() {}
    
    // === 测试4: 带泛型的方法 ===
    public <T> T genericMethod(T input) {
        return input;
    }
    
    public <T extends Comparable<T>> T maxMethod(T a, T b) {
        return a.compareTo(b) > 0 ? a : b;
    }
    
    // === 测试5: 内部类中的方法 ===
    class InnerClass {
        public void innerMethod() {
            System.out.println("inner");
        }
        
        class InnerInner {
            public void deepMethod() {}
        }
    }
    
    // === 测试6: 匿名类中的方法（不应被扫描到顶层）===
    Runnable r = new Runnable() {
        @Override
        public void run() {
            System.out.println("anonymous run");
        }
        
        public void extraMethod() {
            System.out.println("anonymous extra");
        }
    };
    
    // === 测试7: Lambda 表达式（不应被识别为方法）===
    Function<Integer, Integer> lambda = (x) -> x * 2;
    BiFunction<Integer, Integer, Integer> add = (a, b) -> a + b;
    Consumer<String> printer = System.out::println;
    
    // === 测试8: 变参方法 ===
    public void varargsMethod(String... args) {
        for (String arg : args) {
            System.out.println(arg);
        }
    }
    
    // === 测试9: 重载方法 ===
    public void overloaded(int a) {}
    public void overloaded(int a, int b) {}
    public void overloaded(String a) {}
    public String overloaded(double a) { return "overloaded"; }
    
    // === 测试10: 构造方法 ===
    public java_confusing() {}
    public java_confusing(int x) { this.x = x; }
    public java_confusing(String name, int value) { this.name = name; this.value = value; }
    
    private int x;
    private String name;
    private int value;
    
    // === 测试11: getter/setter ===
    public int getX() { return x; }
    public void setX(int x) { this.x = x; }
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    
    // === 测试12: 注解中的方法（JUnit 等）===
    // @Test 注解下的方法
    // 但实际不会在注释中出现
    
    // === 测试13: 返回值类型复杂的方法 ===
    public Map<String, List<Integer>> complexReturnType() {
        return new HashMap<>();
    }
    
    public Optional<String>[] arrayOfGeneric() {
        return new Optional[0];
    }
    
    // === 测试14: 带 throws 的方法 ===
    public void throwingMethod() throws IOException, InterruptedException {
        if (true) throw new IOException();
    }
    
    // === 测试15: 非常长的方法名 ===
    public void thisIsAVeryVeryLongMethodNameThatShouldStillBeDetectedByTheScanner() {}
    
    // === 测试16: 空方法体 ===
    public void emptyBody() {}
    
    // === 测试17: 注解参数中的方法调用（不应被识别）===
    // @SuppressWarnings("unchecked")
    
    // === 测试18: 内部接口中的方法 ===
    interface InnerInterface {
        void interfaceMethod();
        default void defaultMethod() {
            System.out.println("default");
        }
        static void staticInterfaceMethod() {}
    }
    
    // === 测试19: 枚举中的方法 ===
    enum Status {
        PENDING {
            @Override
            public void handle() {
                System.out.println("pending");
            }
        },
        DONE {
            @Override
            public void handle() {
                System.out.println("done");
            }
        };
        
        public abstract void handle();
    }
    
    // 真正的方法
    public void realJavaMethod() {
        System.out.println("real");
    }
}
