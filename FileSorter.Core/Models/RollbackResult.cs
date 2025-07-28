namespace FileSorter.Core.Models;

public class RollbackResult
{
    public bool Success { get; set; }
    public int TotalOperations { get; set; }
    public int SuccessfulRollbacks { get; set; }
    public int FailedRollbacks { get; set; }
    public List<string> Errors { get; set; } = new();
    public TimeSpan Duration { get; set; }
}