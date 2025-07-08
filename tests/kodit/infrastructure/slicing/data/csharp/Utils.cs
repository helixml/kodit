using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;

namespace Utils
{
    public static class CurrencyHelper
    {
        public static string FormatCurrency(decimal amount)
        {
            return amount.ToString("C", CultureInfo.CurrentCulture);
        }

        public static bool TryParseCurrency(string value, out decimal result)
        {
            return decimal.TryParse(value, NumberStyles.Currency, CultureInfo.CurrentCulture, out result);
        }
    }

    public static class StringHelper
    {
        public static List<List<T>> ChunkList<T>(IEnumerable<T> source, int chunkSize)
        {
            if (source == null)
                throw new ArgumentNullException(nameof(source));
            if (chunkSize <= 0)
                throw new ArgumentException("Chunk size must be positive", nameof(chunkSize));

            var result = new List<List<T>>();
            var sourceList = source.ToList();

            for (int i = 0; i < sourceList.Count; i += chunkSize)
            {
                var chunk = sourceList.Skip(i).Take(chunkSize).ToList();
                result.Add(chunk);
            }

            return result;
        }

        public static bool ValidateInput(string value)
        {
            if (string.IsNullOrWhiteSpace(value))
                return false;

            if (double.TryParse(value, out double number))
            {
                return number > 0;
            }

            return false;
        }

        public static string CapitalizeFirst(string input)
        {
            if (string.IsNullOrEmpty(input))
                return input;

            return char.ToUpper(input[0]) + input.Substring(1).ToLower();
        }
    }

    public static class MathHelper
    {
        public static double CalculateCircleArea(double radius)
        {
            if (radius < 0)
                throw new ArgumentException("Radius cannot be negative", nameof(radius));

            return Math.PI * radius * radius;
        }

        public static int Fibonacci(int n)
        {
            if (n < 0)
                throw new ArgumentException("Input must be non-negative", nameof(n));

            if (n <= 1)
                return n;

            return Fibonacci(n - 1) + Fibonacci(n - 2);
        }
    }
}