using System.ComponentModel;
using System.Diagnostics;
using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public class SortingService : ISortingService
{
    public async Task<SortingResult> SortFilesAsync(string directoryPath, SortingProfile profile, CancellationToken cancellationToken = default)
    {
        var validationResult = ValidateDirectory(directoryPath);
        
        if (!validationResult.IsValid)
        {
            return new SortingResult
            { 
                Success = false, 
                Errors = validationResult.Errors 
            };
        }
        
        var scanningOption = CreateScanningOptions(profile);
        
        var allFiles = await GetFilesToSortAsync(directoryPath, scanningOption, cancellationToken);

        var fileGroups = GroupFilesByCriteria(allFiles, profile);

        foreach (var group in fileGroups)
        {
            await ProcessFileGroup(group.Key, group.Value, profile, cancellationToken);
        }
        
        throw new NotImplementedException();
    }

    public ValidationResult ValidateDirectory(string directoryPath)
    {
        throw new NotImplementedException();
    }

    public Task<IEnumerable<FileItem>> GetFilesToSortAsync(string directoryPath, ScanningOptions scanningOptions,
        CancellationToken cancellationToken = default)
    {
        throw new NotImplementedException();
    }

    private static ScanningOptions CreateScanningOptions(SortingProfile sortingProfile)
    {
        throw new NotImplementedException();
    }
    
    private Dictionary<string, List<FileItem>> GroupFilesByCriteria(
        IEnumerable<FileItem> files, SortingProfile profile)
    {
        throw new NotImplementedException();
    }
    
    private static Task ProcessFileGroup(
        string groupKey, 
        List<FileItem> groupValues, 
        SortingProfile profile, 
        CancellationToken cancellationToken = default)
    {
        throw new NotImplementedException();
    }
}