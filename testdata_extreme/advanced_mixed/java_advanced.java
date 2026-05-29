import java.util.*;
import java.util.function.*;

/**
 * 超级测试: Java 局部方法与注释干扰
 * Java 8+ 支持 lambda，内部类可以有方法，这些不应与顶级方法混淆
 */
public class java_advanced {
    
    // ===== 测试1: 普通方法 =====
    public String globalMethodA(String input) {
        return input.toUpperCase();
    }
    
    private int globalMethodB(int x) {
        return x + 1;
    }
    
    // ===== 测试2: 块注释中的方法 =====
    /*
    public void blockCommentedA() {
        System.out.println("should NOT be detected");
    }
    
    private String blockCommentedB() {
        return "should NOT be detected";
    }
    */
    
    // ===== 测试3: 行注释中的方法 =====
    // public void lineCommentedA() {
    //     System.out.println("should NOT be detected");
    // }
    
    // ===== 测试4: 字符串中的方法 =====
    String codeInString = """
        public void insideString() {
            System.out.println("should NOT be detected");
        }
        """;
    
    // ===== 测试5: 内部类中的方法（应检测到）=====
    class InnerHandler {
        public void handleA() {
            System.out.println("inner handle A");
        }
        
        private int handleB(int x) {
            return x * 2;
        }
        
        class DeepInner {
            public void deepMethod() {
                System.out.println("deep");
            }
        }
    }
    
    // ===== 测试6: 匿名类中的方法（应被检测到）=====
    Runnable anonymousRunner = new Runnable() {
        @Override
        public void run() {
            System.out.println("anonymous run");
        }
        
        // 匿名类中的额外方法
        public void extraMethod() {
            System.out.println("anonymous extra");
        }
    };
    
    // ===== 测试7: Lambda（不应被检测为方法）=====
    Function<Integer, Integer> doubler = (x) -> x * 2;
    Function<Integer, Integer> tripler = (x) -> x * 3;
    BiFunction<Integer, Integer, Integer> adder = (a, b) -> a + b;
    Consumer<String> printer = System.out::println;
    Supplier<Double> randomizer = () -> Math.random();
    Predicate<Integer> isEven = n -> n % 2 == 0;
    
    // ===== 测试8: 方法内部定义 lambda（不应被检测）=====
    public List<Integer> processWithLambdas(List<Integer> items) {
        // lambda 在方法内部
        Function<Integer, Integer> processor = (x) -> {
            // lambda 内部有代码块，但本身不是方法
            return x * x;
        };
        
        List<Integer> result = new ArrayList<>();
        for (Integer item : items) {
            result.add(processor.apply(item));
        }
        return result;
    }
    
    // ===== 测试9: 方法引用（不应被检测）=====
    public void methodRefDemo() {
        // 方法引用不是方法定义
        Consumer<String> printRef = System.out::println;
        Function<String, Integer> lengthRef = String::length;
        printRef.accept("test");
    }
    
    // ===== 测试10: 重载方法 =====
    public void overloaded(int a) {}
    public void overloaded(int a, int b) {}
    public void overloaded(String a) {}
    public String overloaded(double a) { return "overloaded"; }
    protected void overloaded(long a) {}
    
    // ===== 测试11: 构造方法 =====
    public java_advanced() {}
    public java_advanced(String name) { this.name = name; }
    public java_advanced(String name, int value) { this.name = name; this.value = value; }
    
    private String name;
    private int value;
    
    // ===== 测试12: 泛型方法 =====
    public <T> T identity(T input) { return input; }
    
    public <T extends Comparable<T>> T maxOf(T a, T b) {
        return a.compareTo(b) > 0 ? a : b;
    }
    
    // ===== 测试13: 变参方法 =====
    public void varargsMethod(String... args) {
        for (String arg : args) {
            System.out.println(arg);
        }
    }
    
    // ===== 测试14: 枚举中的方法 =====
    enum HttpStatus {
        OK(200) {
            @Override
            public String describe() { return "Success"; }
        },
        NOT_FOUND(404) {
            @Override
            public String describe() { return "Not Found"; }
        },
        ERROR(500) {
            @Override
            public String describe() { return "Server Error"; }
        };
        
        public abstract String describe();
        
        private final int code;
        HttpStatus(int code) { this.code = code; }
        public int getCode() { return code; }
    }
    
    // ===== 测试15: 注释中的大段代码 =====
    /* 以下代码是注释，不应被扫描
    public class FakeService {
        public void doSomething() {
            // 假装做了些事情
        }
        
        private String processData(String input) {
            return "processed: " + input;
        }
        
        protected int calculate(int a, int b, int c) {
            return (a + b) * c;
        }
        
        public static FakeService createInstance() {
            return new FakeService();
        }
    }
    */
    
    // ===== 测试16: 接口中的默认方法和静态方法 =====
    interface DataProcessor {
        void process();
        
        default void preProcess() {
            System.out.println("pre-processing");
        }
        
        default void postProcess() {
            System.out.println("post-processing");
        }
        
        static DataProcessor create() {
            return () -> System.out.println("processing");
        }
    }
    
    // ===== 测试17: 内部接口 =====
    interface Callback {
        void onSuccess(String data);
        void onError(Exception e);
        
        default void onFinally() {
            System.out.println("finally");
        }
    }
    
    // ===== 测试18: 大的注释块，多语言混合 =====
    /* 
    以下包含多语言代码，都在注释中：
     
    // Go code
    func helper() string {
        return "helper"
    }
    
    # Python code
    def helper():
        return "helper"
    
    // JS code
    function helper() {
        return "helper";
    }
    */
    
    // ===== 测试19: 条件编译（Java 没有，但测试注释中的混淆）=====
    // #ifdef DEBUG
    // public void debugMethod() {
    //     System.out.println("debug");
    // }
    // #endif
    
    // ===== 测试20: 最终真实方法 =====
    public String ultimateRealJavaMethod() {
        return "I am real";
    }
}
