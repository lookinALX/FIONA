using CommandLine;
using FileSorter.CLI.Commands;
using FileSorter.Core.Services;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace FileSorter.CLI;

class Program
{
    static async Task<int> Main(string[] args)
    {
        // If no arguments are provided, run interactive mode
        if (args.Length == 0)
        {
            return await RunInteractiveMode();
        }

        // If arguments are provided, process them as usual
        Console.WriteLine($"Raw arguments: [{string.Join(", ", args)}]");
        return await ProcessArguments(args);
    }

    private static async Task<int> RunInteractiveMode()
    {
        Console.WriteLine("FileSorter CLI - Enter command to start:");
        Console.WriteLine("Type 'fiona' for help or 'exit' to quit");
        
        while (true)
        {
            Console.Write("> ");
            var input = Console.ReadLine()?.Trim();
            
            if (string.IsNullOrEmpty(input))
                continue;
                
            if (input.Equals("exit", StringComparison.OrdinalIgnoreCase) || 
                input.Equals("quit", StringComparison.OrdinalIgnoreCase))
            {
                Console.WriteLine("Goodbye!");
                return 0;
            }
            
            if (input.Equals("fiona", StringComparison.OrdinalIgnoreCase) ||
                input.Equals("help", StringComparison.OrdinalIgnoreCase))
            {
                ShowCommands();
                continue;
            }
            
            // Parse the entered command into arguments
            var commandArgs = ParseCommandLine(input);
            if (commandArgs.Length > 0)
            {
                var result = await ProcessArguments(commandArgs);
                if (result != 0)
                {
                    Console.WriteLine($"Command finished with code: {result}");
                }
            }
            else
            {
                Console.WriteLine("Unknown command. Type 'fiona' for command list or 'exit' to quit.");
            }
        }
    }

    private static void ShowCommands()
    {
        Console.WriteLine("\n=== FIONA - File Organizer Commands ===");
        Console.WriteLine();
        
        Console.WriteLine("FILE GROUPING:");
        Console.WriteLine("  group --source <path> --by <criteria> [options]");
        Console.WriteLine("    Groups files by specified criteria");
        Console.WriteLine();
        
        Console.WriteLine("GROUPING CRITERIA (--by):");
        Console.WriteLine("  • extension, ext     - by file extension");
        Console.WriteLine("  • date, oldest       - by oldest date");
        Console.WriteLine("  • creationdate       - by creation date");
        Console.WriteLine("  • modificationdate   - by modification date");
        Console.WriteLine("  • category           - by file category");
        Console.WriteLine();
        
        Console.WriteLine("ADDITIONAL OPTIONS:");
        Console.WriteLine("  --source <path>           - source directory (default: current directory)");
        Console.WriteLine("  --with <criteria>         - secondary grouping");
        Console.WriteLine("  --how <copy|move>         - copy or move files (default: copy)");
        Console.WriteLine("  --conflict <strategy>     - conflict resolution (skip|overwrite|rename|ask|keepboth)");
        Console.WriteLine("  --primary-date <option>   - date grouping (year|month|yearmonth)");
        Console.WriteLine("  --backup                  - create backups when overwriting");
        Console.WriteLine("  --dry-run                 - show what will be done without executing");
        Console.WriteLine();
        
        Console.WriteLine("ROLLBACK OPERATIONS:");
        Console.WriteLine("  rollback [--force]");
        Console.WriteLine("    Rolls back the last grouping operation");
        Console.WriteLine();
        
        Console.WriteLine("USAGE EXAMPLES:");
        Console.WriteLine("  group --by extension");
        Console.WriteLine("  group --source C:\\MyFiles --by category --how move");
        Console.WriteLine("  group --by date --primary-date year --dry-run");
        Console.WriteLine("  group --by extension --with category");
        Console.WriteLine("  rollback --force");
        Console.WriteLine();
        
        Console.WriteLine("SYSTEM COMMANDS:");
        Console.WriteLine("  fiona, help  - show this command list");
        Console.WriteLine("  exit, quit   - exit the program");
        Console.WriteLine();
        
        Console.WriteLine("QUICK START:");
        Console.WriteLine("  1. Navigate to folder with files: cd C:\\MyFiles");
        Console.WriteLine("  2. Run: fiona");
        Console.WriteLine("  3. Type: group --by extension --dry-run");
        Console.WriteLine("  4. If satisfied, run: group --by extension");
        Console.WriteLine();
    }

    private static string[] ParseCommandLine(string commandLine)
    {
        var args = new List<string>();
        var current = "";
        var inQuotes = false;
        
        for (int i = 0; i < commandLine.Length; i++)
        {
            var c = commandLine[i];
            
            if (c == '"')
            {
                inQuotes = !inQuotes;
            }
            else if (c == ' ' && !inQuotes)
            {
                if (!string.IsNullOrEmpty(current))
                {
                    args.Add(current);
                    current = "";
                }
            }
            else
            {
                current += c;
            }
        }
        
        if (!string.IsNullOrEmpty(current))
        {
            args.Add(current);
        }
        
        return args.ToArray();
    }

    private static async Task<int> ProcessArguments(string[] args)
    {
        // Create host with DI container
        var host = CreateHostBuilder(args).Build();
        
        var result = await Parser.Default.ParseArguments<GroupOptions, RollbackOptions>(args)
            .MapResult(
                (GroupOptions opts) => {
                    Console.WriteLine($"Parsed SourceDirectory: '{opts.SourceDirectory}'");
                    Console.WriteLine($"Parsed GroupBy: '{opts.GroupBy}'");
                    
                    var groupCommands = host.Services.GetRequiredService<GroupCommands>();
                    return groupCommands.ExecuteAsync(opts);
                },
                (RollbackOptions opts) => {
                    var rollbackCommands = host.Services.GetRequiredService<RollbackCommands>();
                    return rollbackCommands.ExecuteAsync(opts);
                },
                errs => {
                    Console.WriteLine("Failed to parse command line arguments:");
                    foreach (var error in errs)
                    {
                        Console.WriteLine($"  {error}");
                    }
                    Console.WriteLine("\nType 'fiona' for help with available commands.");
                    return Task.FromResult(1);
                });
        
        return result;
    }

    private static IHostBuilder CreateHostBuilder(string[] args) =>
        Host.CreateDefaultBuilder(args)
            .ConfigureServices((context, services) =>
            {
                // Register core services
                services.AddSingleton<IRollbackService, RollbackService>();
                services.AddSingleton<IConflictResolver, ConflictResolver>();
                services.AddTransient<IGroupingService, GroupingService>();
                
                // Register command handlers
                services.AddTransient<GroupCommands>();
                services.AddTransient<RollbackCommands>();
            })
            .ConfigureLogging(logging =>
            {
                logging.ClearProviders();
                logging.AddConsole();
                logging.SetMinimumLevel(LogLevel.Warning); // Only show warnings and errors
            });
}