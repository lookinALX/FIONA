using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public interface IGroupingService
{
    Task<GroupingResult> GroupFilesAsync(string directoryPath, GroupingProfile profile, CancellationToken cancellationToken = default);
    
    Task<IEnumerable<FileItem>> GetFilesToGroupAsync(string directoryPath, CancellationToken cancellationToken = default);
}

public class ScanningOptions
{
    
}