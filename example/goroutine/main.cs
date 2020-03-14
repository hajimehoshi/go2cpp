// SPDX-License-Identifier: Apache-2.0

using System;
using Go2DotNet.Example.Goroutine.AutoGen;

namespace Go2DotNet.Example.Goroutine
{
    class Program
    {
        static void Main(string[] args)
        {
            Go go = new Go();
            go.Run(args);
        }
    }    
}
