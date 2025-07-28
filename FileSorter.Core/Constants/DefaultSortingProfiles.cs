using FileSorter.Core.Models;

namespace FileSorter.Core.Constants;

public class DefaultSortingProfiles
{
    public static GroupingProfile ByExtension => new()
    {
        PrimaryCriteria = GroupingCriteria.Extension,
        SecondaryCriteria = GroupingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.None,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile ByFileType => new()
    {
        PrimaryCriteria = GroupingCriteria.FileCategory,
        SecondaryCriteria = GroupingCriteria.Extension,
        DatePrimaryGroupingOption = DateGroupingOption.None,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile ByCreationYear => new()
    {
        PrimaryCriteria = GroupingCriteria.CreationDate,
        SecondaryCriteria = GroupingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile ByOldestYear => new()
    {
        PrimaryCriteria = GroupingCriteria.OldestDate,
        SecondaryCriteria = GroupingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile ByModificationYear => new()
    {
        PrimaryCriteria = GroupingCriteria.ModificationDate,
        SecondaryCriteria = GroupingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.None,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile ByCreationYearThenMonth => new()
    {
        PrimaryCriteria = GroupingCriteria.CreationDate,
        SecondaryCriteria = GroupingCriteria.CreationDate,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.Month,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile ByModificationYearThenMonth => new()
    {
        PrimaryCriteria = GroupingCriteria.ModificationDate,
        SecondaryCriteria = GroupingCriteria.None,
        DatePrimaryGroupingOption = DateGroupingOption.Year,
        DateSecondaryGroupingOption = DateGroupingOption.Month,
        FileOperationType = FileOperationType.Move
    };
    
    public static GroupingProfile TypeThenYear => new()
    {
        PrimaryCriteria = GroupingCriteria.FileCategory,
        SecondaryCriteria = GroupingCriteria.CreationDate,
        DatePrimaryGroupingOption = DateGroupingOption.None,
        DateSecondaryGroupingOption = DateGroupingOption.Year,
        FileOperationType = FileOperationType.Move
    };
}