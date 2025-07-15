namespace FileSorter.Core.Models;

public class ConflictHandling
{
    public ConflictResolutionStrategy Strategy { get; set; } = ConflictResolutionStrategy.Rename;
    public bool BackupOriginal { get; set; } = false;
    public string BackupSuffix { get; set; } = "_backup";
}

public enum ConflictResolutionStrategy
{
    Skip,
    Overwrite,
    Rename,
    Ask,
    KeepBoth
}