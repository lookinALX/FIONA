namespace FileSorter.Core.Models;

public class FileOperationRecord
{
    public string SourcePath { get; set; } = string.Empty;
    public string DestinationPath { get; set; } = string.Empty;
    public FileOperationType OperationType { get; set; }
    public DateTime Timestamp { get; set; }
    public bool Success { get; set; }
    public string? BackupPath { get; set; }
    public string? Error { get; set; }
    public ConflictResolutionStrategy? ConflictResolution { get; set; }
}