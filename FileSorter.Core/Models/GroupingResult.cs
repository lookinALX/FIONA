namespace FileSorter.Core.Models;

public class GroupingResult
{
    public bool Success { get; set; }
    public int ProcessedFiles { get; set; }
    public int SkippedFiles { get; set; }
    public int FailedFiles { get; set; }
    public TimeSpan Duration { get; set; }
    public List<string> Errors { get; set; } = new();
    public Dictionary<string, List<string>> DestinationPathDict { get; set; } = new();
    public List<string> DestinationPathList { get; set; } = new();    
}