using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public class ConflictResolver : IConflictResolver
{
    public async Task<ConflictResolutionResult> ResolveConflictAsync(
        string sourcePath, 
        string destinationPath, 
        ConflictHandling conflictHandling,
        CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(sourcePath);
        ArgumentException.ThrowIfNullOrWhiteSpace(destinationPath);
        ArgumentNullException.ThrowIfNull(conflictHandling);

        if (!File.Exists(destinationPath))
        {
            return new ConflictResolutionResult
            {
                ShouldProceed = true,
                FinalDestinationPath = destinationPath,
                AppliedStrategy = conflictHandling.Strategy
            };
        }

        return conflictHandling.Strategy switch
        {
            ConflictResolutionStrategy.Skip => await HandleSkipAsync(destinationPath),
            ConflictResolutionStrategy.Overwrite => await HandleOverwriteAsync(sourcePath, destinationPath, conflictHandling, cancellationToken),
            ConflictResolutionStrategy.Rename => await HandleRenameAsync(destinationPath),
            ConflictResolutionStrategy.KeepBoth => await HandleKeepBothAsync(destinationPath),
            ConflictResolutionStrategy.Ask => await HandleAskAsync(sourcePath, destinationPath),
            _ => throw new NotSupportedException($"Conflict resolution strategy {conflictHandling.Strategy} is not supported")
        };
    }
    
    private static Task<ConflictResolutionResult> HandleSkipAsync(string destinationPath)
    {
        return Task.FromResult(new ConflictResolutionResult
        {
            ShouldProceed = false,
            FinalDestinationPath = destinationPath,
            AppliedStrategy = ConflictResolutionStrategy.Skip,
            Message = "File skipped due to conflict"
        });
    }
    
    private static async Task<ConflictResolutionResult> HandleOverwriteAsync(
        string sourcePath, 
        string destinationPath, 
        ConflictHandling conflictHandling, 
        CancellationToken cancellationToken)
    {
        string? backupPath = null;
        
        if (conflictHandling.BackupOriginal)
        {
            backupPath = GenerateBackupPath(destinationPath, conflictHandling.BackupSuffix);
            await Task.Run(() => File.Copy(destinationPath, backupPath), cancellationToken);
        }

        return new ConflictResolutionResult
        {
            ShouldProceed = true,
            FinalDestinationPath = destinationPath,
            AppliedStrategy = ConflictResolutionStrategy.Overwrite,
            BackupPath = backupPath,
            Message = conflictHandling.BackupOriginal ? "File will be overwritten with backup created" : "File will be overwritten"
        };
    }
    
    private static Task<ConflictResolutionResult> HandleRenameAsync(string destinationPath)
    {
        var newPath = GenerateUniqueFileName(destinationPath);
        
        return Task.FromResult(new ConflictResolutionResult
        {
            ShouldProceed = true,
            FinalDestinationPath = newPath,
            AppliedStrategy = ConflictResolutionStrategy.Rename,
            Message = $"File renamed to avoid conflict: {Path.GetFileName(newPath)}"
        });
    }
    
    private static Task<ConflictResolutionResult> HandleKeepBothAsync(string destinationPath)
    {
        var newPath = GenerateUniqueFileName(destinationPath);
        
        return Task.FromResult(new ConflictResolutionResult
        {
            ShouldProceed = true,
            FinalDestinationPath = newPath,
            AppliedStrategy = ConflictResolutionStrategy.KeepBoth,
            Message = $"Both files kept, new file: {Path.GetFileName(newPath)}"
        });
    }
    
    private static Task<ConflictResolutionResult> HandleAskAsync(string sourcePath, string destinationPath)
    {
        Console.WriteLine($"File conflict detected:");
        Console.WriteLine($"Source: {sourcePath}");
        Console.WriteLine($"Destination: {destinationPath}");
        Console.WriteLine("Choose action: (s)kip, (o)verwrite, (r)ename, (k)eep both");
        
        while (true)
        {
            var input = Console.ReadKey(true).KeyChar.ToString().ToLowerInvariant();
            
            switch (input)
            {
                case "s":
                    return HandleSkipAsync(destinationPath);
                case "o":
                    return Task.FromResult(new ConflictResolutionResult
                    {
                        ShouldProceed = true,
                        FinalDestinationPath = destinationPath,
                        AppliedStrategy = ConflictResolutionStrategy.Overwrite,
                        Message = "User chose to overwrite"
                    });
                case "r":
                    return HandleRenameAsync(destinationPath);
                case "k":
                    return HandleKeepBothAsync(destinationPath);
                default:
                    Console.WriteLine("Invalid choice. Please select s, o, r, or k.");
                    continue;
            }
        }
    }
    
    private static string GenerateUniqueFileName(string filePath)
    {
        var directory = Path.GetDirectoryName(filePath) ?? throw new ArgumentException("Invalid file path");
        var fileNameWithoutExtension = Path.GetFileNameWithoutExtension(filePath);
        var extension = Path.GetExtension(filePath);
        
        var counter = 1;
        string newPath;
        
        do
        {
            var newFileName = $"{fileNameWithoutExtension}_{counter}{extension}";
            newPath = Path.Combine(directory, newFileName);
            counter++;
        } 
        while (File.Exists(newPath));
        
        return newPath;
    }

    private static string GenerateBackupPath(string filePath, string backupSuffix)
    {
        var directory = Path.GetDirectoryName(filePath) ?? throw new ArgumentException("Invalid file path");
        var fileNameWithoutExtension = Path.GetFileNameWithoutExtension(filePath);
        var extension = Path.GetExtension(filePath);
        
        var timestamp = DateTime.Now.ToString("yyyyMMdd_HHmmss");
        var backupFileName = $"{fileNameWithoutExtension}{backupSuffix}_{timestamp}{extension}";
        
        return Path.Combine(directory, backupFileName);
    }
}