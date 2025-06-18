using FileSorter.Core.Models;

namespace FileSorter.Core.Constants;

public class DefaultSortingProfiles
{
    public static SortingProfile ByExtension => new()
    {
        PrimaryCriteria = SortingCriteria.Extension,
        SecondaryCriteria = SortingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.None,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static SortingProfile ByFileType => new()
    {
        PrimaryCriteria = SortingCriteria.FileCategory,
        SecondaryCriteria = SortingCriteria.Extension,
        DatePrimaryGroupingOption = DateGroupingOption.None,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static SortingProfile ByCreationYear => new()
    {
        PrimaryCriteria = SortingCriteria.CreationDate,
        SecondaryCriteria = SortingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static SortingProfile ByModificationYear => new()
    {
        PrimaryCriteria = SortingCriteria.ModificationDate,
        SecondaryCriteria = SortingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static SortingProfile ByCreationYearThenMonth => new()
    {
        PrimaryCriteria = SortingCriteria.CreationDate,
        SecondaryCriteria = SortingCriteria.CreationDate,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.Month,
        FileOperationType = FileOperationType.Move
    };
    
    public static SortingProfile ByModificationYearThenMonth => new()
    {
        PrimaryCriteria = SortingCriteria.ModificationDate,
        SecondaryCriteria = SortingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.Month,
        FileOperationType = FileOperationType.Move
    };
    
    public static SortingProfile TypeThenYear => new()
    {
        PrimaryCriteria = SortingCriteria.FileCategory,
        SecondaryCriteria = SortingCriteria.CreationDate,
        DatePrimaryGroupingOption = DateGroupingOption.None,
        DateSecondaryGroupingOption = DateGroupingOption.Year,
        FileOperationType = FileOperationType.Move
    };
}