using FileSorter.Core.Models;

namespace FileSorter.Core.Helpers;

public static class FunctionHelper
{
    public static string[] ExtractFileExtensions(this string[] files)
    {
        return files.Select(file => Path.GetExtension(file).TrimStart('.'))
            .Where(ext => !string.IsNullOrEmpty(ext))
            .ToArray();
    }

    public static DateTime[] ExtractFileDateTimes(this string[] files)
    {
        var dates = files.Select(File.GetLastWriteTime).ToArray();
        return dates;
    }

    public static Dictionary<string, List<string>> SplitDateTimeInMonthAndYear(this string[] files)
    {
        var dates = files.ExtractFileDateTimes();
        var dict = dates.GroupBy(date => date.Year.ToString()).ToDictionary(
            group => group.Key,
            group => group.Select(date => date.Month.ParseNumberWithMonth())
                .Distinct().ToList()
        );
        return dict;
    }

    public static string ParseNumberWithMonth(this int number)
    {
        if (Enum.IsDefined(typeof(Month), number))
            return Enum.GetName((Month)number) ?? throw new InvalidOperationException();
        throw new ArgumentException($"Invalid month number: {number}");
    }

    public static GroupingCriteria ParseGroupingCriteria(this string criteria)
    {
        if (string.IsNullOrWhiteSpace(criteria))
            return GroupingCriteria.None;
        
        switch (criteria.Trim().ToLowerInvariant())
        {
            case "none":
                return GroupingCriteria.None;
            case "creationdate":
            case "crdate":
                return GroupingCriteria.CreationDate;
            case "modificationdate":
            case "moddate":
                return GroupingCriteria.ModificationDate;
            case "date":
            case "oldest":
            case "oldestdate":    
                return GroupingCriteria.OldestDate;
            case "extension":
            case "ext":
                return GroupingCriteria.Extension;
            case "filecategory":
            case "category":
            case "cat":
                return GroupingCriteria.FileCategory;
            default:
                throw new ArgumentException($"Invalid grouping criteria: '{criteria}'");
        }
    }

    public static DateGroupingOption ParseGroupingOption(this string criteria)
    {
        if (string.IsNullOrWhiteSpace(criteria) || criteria.Trim().ToLowerInvariant() == "none")
            return DateGroupingOption.None;

        switch (criteria.Trim().ToLowerInvariant())
        {
            case "year":
            case "years":
                return DateGroupingOption.Year;
            case "month":
            case "months":
                return DateGroupingOption.Month;
            case "yearmonth":
            case "year_month":
                return DateGroupingOption.YearMonth;
            default:
                throw new ArgumentException($"Invalid grouping option: '{criteria}'");
        }
    }

    public static FileOperationType ParseFileOperationType(this string criteria)
    {
        if (string.IsNullOrWhiteSpace(criteria) || criteria.Trim().ToLowerInvariant() == "none")
            return FileOperationType.Copy;
        
        switch (criteria.Trim().ToLowerInvariant())
        {
            case "copy":
                return FileOperationType.Copy;
            case "move":
                return FileOperationType.Move;
            default:
                throw new ArgumentException($"Invalid grouping option: '{criteria}'");
        }
    }
}