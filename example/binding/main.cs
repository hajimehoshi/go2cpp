// SPDX-License-Identifier: Apache-2.0

#pragma warning disable 649

using System;
using Go2DotNet.Example.Binding.AutoGen;

namespace Go2DotNet.Example.Binding
{
    public class Test
    {
        internal static void CallTwice(IInvokable a)
        {
            a.Invoke(null);
            a.Invoke(null);
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
