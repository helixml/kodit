using System;
using System.Collections.Generic;
using Models;
using Utils;

namespace TestProject
{
    public class Program
    {
        public static void Main(string[] args)
        {
            Console.WriteLine("=== Shopping Cart Demo ===");
            DemonstrateCart();

            Console.WriteLine("\n=== Point Distance Demo ===");
            DemonstratePoints();

            Console.WriteLine("\n=== String Utils Demo ===");
            DemonstrateStringUtils();
        }

        private static void DemonstrateCart()
        {
            var cart = new ShoppingCart();
            var product1 = new Product("Laptop", 999.99m, 5);
            var product2 = new Product("Mouse", 29.99m, 10);

            cart.AddItem(product1);
            cart.AddItem(product2);

            var total = cart.CalculateTotal();
            Console.WriteLine($"Cart total: {CurrencyHelper.FormatCurrency(total)}");
            Console.WriteLine($"Items in cart: {cart.GetItemCount()}");
        }

        private static void DemonstratePoints()
        {
            var p1 = new Point(0, 0);
            var p2 = new Point(3, 4);

            var distance = p1.DistanceTo(p2);
            Console.WriteLine($"Distance between points: {distance:F2}");
        }

        private static void DemonstrateStringUtils()
        {
            var numbers = new List<int> { 1, 2, 3, 4, 5, 6, 7, 8, 9, 10 };
            var chunks = StringHelper.ChunkList(numbers, 3);
            Console.WriteLine($"Chunked {string.Join(",", numbers)} into: {string.Join(",", chunks.Select(c => $"[{string.Join(",", c)}]"))}");
        }
    }
}