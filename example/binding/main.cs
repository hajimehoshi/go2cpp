// SPDX-License-Identifier: Apache-2.0

#pragma warning disable 649

using System;
using Go2DotNet.Example.Binding.AutoGen;

namespace Go2DotNet.Example.Binding
{
    public class External
    {
        // All numerics from Go are treated as double due to syscall/js.
        internal static double StaticField;

        internal static double StaticProperty
        {
            get { return staticProperty; }
            set { staticProperty = value; }
        }
        static double staticProperty;

        internal static void StaticMethod(string arg)
        {
            Console.WriteLine($"arg: {arg}");
        }

        public External(string str, double num)
        {
            this.str = str;
            this.num = num;
        }

        internal double InstanceField;

        internal double InstanceProperty
        {
            get { return instanceProperty; }
            set { instanceProperty = value; }
        }
        private double instanceProperty;

        internal void InstanceMethod()
        {
            Console.WriteLine($"str: {this.str}, num: {this.num}");
        }

        internal object InvokeGo(object a, string arg)
        {
            // TODO: Better interface for a function.
            return Go2DotNet.Example.Binding.AutoGen.JSObject.ReflectApply(a, null, new object[] { arg });
        }

        private string str;
        private double num;
    }

    class Program
    {
        static void Main(string[] args)
        {
            Go go = new Go();
            go.Run(args);
        }
    }    
}
