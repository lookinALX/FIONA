namespace FileSorter.Core.Models;

public class ConflictResolutionResult
{
    public bool ShouldProceed { get; set; }
    public string FinalDestinationPath { get; set; } = string.Empty;
    public ConflictResolutionStrategy AppliedStrategy { get; set; }
    public string? BackupPath { get; set; }
    public string? Message { get; set; }
}