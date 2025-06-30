using CommandLine;

namespace FileSorter.CLI.Commands;

[Verb("group", HelpText = "Group files by criteria")]
public class GroupOptions
{
    [Value(0, MetaName = "source", Required = true, HelpText = "Path to source files")]
    public string SourceDirectory { get; set; } = string.Empty;
    
    [Option('b', "by", Default = "date", HelpText = "Group by: extension, date (oldest), " +
                                                    "creation date, modify date, category, size")]
    public string GroupBy { get; set; } = string.Empty;
    
    [Option('w', "with", Default = "none", HelpText = "Secondary grouping option")]
    public string GroupWith { get; set; } = string.Empty;
    
    [Option('h', "how", Default = "copy", HelpText = "How to handle files, copy or move")]
    public string HowToHandle { get; set; } = string.Empty;
    
    [Option('o', "output", HelpText = "Output directory (default: source directory)")]
    public string? OutputDirectory { get; set; }
}