using CommandLine;
using FileSorter.CLI.Commands;
using FileSorter.Core.Services;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Options;

namespace FileSorter.CLI;

class Program
{
    static async Task<int> Main(string[] args)
    {
        Console.WriteLine($"Raw arguments: [{string.Join(", ", args)}]");
    
        var host = CreateHostBuilder(args).Build();

        return await Parser.Default.ParseArguments<GroupOptions>(args)
            .MapResult(
                (GroupOptions opts) => {
                    Console.WriteLine($"Parsed SourceDirectory: '{opts.GetEffectiveSourceDirectory()}'");
                    Console.WriteLine($"Parsed GroupBy: '{opts.GroupBy}'");
                    return host.Services.GetRequiredService<GroupCommands>().ExecuteAsync(opts);
                },
                errs => {
                    Console.WriteLine("Parsing failed!");
                    foreach(var err in errs) {
                        Console.WriteLine($"Error: {err}");
                    }
                    return Task.FromResult(1);
                }
            );
    }

    static IHostBuilder CreateHostBuilder(string[] args) =>
        Host.CreateDefaultBuilder(args)
            .ConfigureServices((context, services) =>
            {
                services.AddSingleton<GroupCommands>();

                services.AddTransient<IGroupingService, GroupingService>();
            });
}