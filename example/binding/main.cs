// SPDX-License-Identifier: Apache-2.0

using System;
using Go2DotNet.Example.Binding.AutoGen;

namespace Go2DotNet.Example.Binding
{
    class External
    {
        internal static int StaticField = 1;

        internal static int StaticProperty
        {
            get { return staticProperty; }
            set { staticProperty = value; }
        }
        static int staticProperty = 2;

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
