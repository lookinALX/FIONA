using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public interface ISortingService
{
    Task<SortingResult> SortFilesAsync(
        string directoryPath, SortingProfile profile, CancellationToken cancellationToken = default);
    
    ValidationResult ValidateDirectory(string directoryPath);
    
    Task<IEnumerable<FileItem>> GetFilesToSortAsync(
        string directoryPath, ScanningOptions scanningOptions, CancellationToken cancellationToken = default);
}

public class ScanningOptions
{
    
}