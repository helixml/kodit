using System;
using System.Collections.Generic;
using System.IO;

namespace Kodit.CodeGraph
{
    public static class Helper
    {
        public static string HelperFunction(List<string> x)
        {
            return string.Join(" ", x);
        }
    }

    public class MyClass
    {
        private int value;

        public MyClass(int value)
        {
            this.value = value;
        }

        public List<string> GetValue()
        {
            return new List<string>(Directory.GetFiles(Directory.GetCurrentDirectory()));
        }

        public void PrintValue()
        {
            Console.WriteLine(value);
        }
    }

    public class Program
    {
        public static string Main()
        {
            var obj = new MyClass(42);
            var result = Helper.HelperFunction(obj.GetValue());
            return result;
        }
    }
} 