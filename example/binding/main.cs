// SPDX-License-Identifier: Apache-2.0

#pragma warning disable 649

using System;
using Go2DotNet.Example.Binding.AutoGen;

namespace Go2DotNet.Example.Binding
{
    class External
    {
        // All numerics from Go are treated as double due to syscall/js.
        internal static double StaticField;

        internal static double StaticProperty
        {
            get { return staticProperty; }
            set { staticProperty = value; }
        }
        static double staticProperty;

        public static void StaticMethod(string arg)
        {
            Console.WriteLine(arg);
        }
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
