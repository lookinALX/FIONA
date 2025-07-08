using CommandLine;

namespace FileSorter.CLI.Commands;

[Verb("group", HelpText = "Group files by criteria")]
public class GroupOptions
{
    [Option('s', "source", Default = null, HelpText = "Path to source files (defaults to current directory)")]
    public string? SourceDirectory { get; set; }
    
    public string GetEffectiveSourceDirectory() =>
        string.IsNullOrWhiteSpace(SourceDirectory) ? Directory.GetCurrentDirectory() : SourceDirectory;
    
    [Option('b', "by", Default = "date", HelpText = "Group by: extension, date (oldest), " +
                                                    "creation date, modify date, category, size")]
    public string GroupBy { get; set; } = string.Empty;
    
    [Option('w', "with", Default = "none", HelpText = "Secondary grouping option")]
    public string GroupWith { get; set; } = string.Empty;
    
    [Option('h', "how", Default = "copy", HelpText = "How to handle files, copy or move")]
    public string HowToHandle { get; set; } = string.Empty;
    
    [Option('o', "output", HelpText = "Output directory (default: source directory)")]
    public string? OutputDirectory { get; set; }
    
    [Option('p', "primary date", Default = "none", HelpText = "Date grouping option (year, month, year month, default: none)")]
    public string DateGroupingPrimary { get; set; } = string.Empty;
    
    [Option('d', "secondary date", Default = "none", HelpText = "Date grouping option (year, month, year month, default: none)")]
    public string DateGroupingSecondary { get; set; } = string.Empty;
}