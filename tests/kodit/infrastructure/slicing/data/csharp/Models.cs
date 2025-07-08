using System;
using System.Collections.Generic;
using System.Linq;

namespace Models
{
    public class Product
    {
        public string Name { get; set; }
        public decimal Price { get; set; }
        public int Inventory { get; set; }

        public Product(string name, decimal price, int inventory = 0)
        {
            ValidatePositive(price, nameof(price));
            if (inventory < 0)
                throw new ArgumentException("Inventory cannot be negative");

            Name = name ?? throw new ArgumentNullException(nameof(name));
            Price = price;
            Inventory = inventory;
        }

        private static void ValidatePositive(decimal value, string paramName)
        {
            if (value <= 0)
                throw new ArgumentException($"{paramName} must be positive", paramName);
        }
    }

    public class ShoppingCart
    {
        private readonly List<Product> _items;

        public ShoppingCart()
        {
            _items = new List<Product>();
        }

        public void AddItem(Product product)
        {
            if (product == null)
                throw new ArgumentNullException(nameof(product));
            _items.Add(product);
        }

        public void RemoveItem(Product product)
        {
            _items.Remove(product);
        }

        public decimal CalculateTotal()
        {
            return _items.Sum(item => item.Price);
        }

        public int GetItemCount()
        {
            return _items.Count;
        }

        public List<Product> GetItems()
        {
            return new List<Product>(_items);
        }
    }

    public class Point
    {
        public double X { get; set; }
        public double Y { get; set; }

        public Point(double x, double y)
        {
            X = x;
            Y = y;
        }

        public double DistanceTo(Point other)
        {
            if (other == null)
                throw new ArgumentNullException(nameof(other));

            var dx = X - other.X;
            var dy = Y - other.Y;
            return Math.Sqrt(dx * dx + dy * dy);
        }
    }
}